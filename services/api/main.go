package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

//go:embed fixtures.json
var fixtureData []byte

type Localized struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

func (l Localized) Text(locale string) string {
	if locale == "zh" {
		return l.ZH
	}
	return l.EN
}

type Knowledge struct {
	ID       string    `json:"id"`
	Title    Localized `json:"title"`
	Keywords []string  `json:"keywords"`
	Answer   Localized `json:"answer"`
}
type Fixtures struct {
	Knowledge []Knowledge          `json:"knowledge"`
	Personas  map[string]Localized `json:"personas"`
}
type Room struct {
	State   string `json:"state"`
	Version int    `json:"version"`
}
type RoomInput struct {
	Action  string `json:"action"`
	Version int    `json:"expectedVersion"`
	EventID string `json:"eventId"`
}
type EventInput struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Payload         string `json:"payload"`
	SimulateFailure bool   `json:"simulateFailure"`
}
type RoomReceipt struct {
	Input  RoomInput
	Result Room
}
type Service struct {
	mu           sync.Mutex
	room         Room
	roomReceipts map[string]RoomReceipt
	events       map[string]EventInput
	attempts     map[string]int
	fixtures     Fixtures
}

func NewService() *Service {
	s := &Service{room: Room{"idle", 1}, roomReceipts: map[string]RoomReceipt{}, events: map[string]EventInput{}, attempts: map[string]int{}}
	if err := json.Unmarshal(fixtureData, &s.fixtures); err != nil {
		panic(err)
	}
	return s
}
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, code string) {
	jsonResponse(w, status, map[string]string{"error": code})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		fail(w, 415, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		fail(w, 400, "invalid_json")
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, "trailing_json")
		return false
	}
	return true
}
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /demo-api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"mode": "fixture", "backend": "Go", "version": "3"})
	})
	mux.HandleFunc("POST /demo-api/chat", s.Chat)
	mux.HandleFunc("GET /demo-api/room", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		jsonResponse(w, 200, s.room)
	})
	mux.HandleFunc("POST /demo-api/room", s.UpdateRoom)
	mux.HandleFunc("POST /demo-api/events", s.AcceptEvent)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://localhost:4321" && origin != "http://127.0.0.1:4321" && origin != "http://127.0.0.1:4738" && origin != "http://localhost:4322" && origin != "http://127.0.0.1:4322" {
			fail(w, 403, "local_origin_required")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Service) Chat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Message string `json:"message"`
		Locale  string `json:"locale"`
		Persona string `json:"persona"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	if len([]rune(input.Message)) == 0 || len([]rune(input.Message)) > 700 {
		fail(w, 422, "message_length")
		return
	}
	if input.Locale != "en" && input.Locale != "zh" {
		fail(w, 422, "locale_required")
		return
	}
	answer := "I will route this question to a person with the conversation context."
	if input.Locale == "zh" {
		answer = "这类问题将转交人工，并保留当前会话上下文。"
	}
	source := ""
	title := ""
	handoff := true
	if input.Persona != "" {
		persona, ok := s.fixtures.Personas[input.Persona]
		if !ok {
			fail(w, 422, "unknown_persona")
			return
		}
		answer = persona.Text(input.Locale)
		handoff = false
	} else {
		for _, k := range s.fixtures.Knowledge {
			found := false
			for _, word := range k.Keywords {
				if strings.Contains(strings.ToLower(input.Message), word) {
					found = true
					break
				}
			}
			if found {
				answer = k.Answer.Text(input.Locale)
				source = k.ID
				title = k.Title.Text(input.Locale)
				handoff = false
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]any{"type": "start", "provider": "fixture"})
	if flusher != nil {
		flusher.Flush()
	}
	runes := []rune(answer)
	for i := 0; i < len(runes); i += 12 {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		end := i + 12
		if end > len(runes) {
			end = len(runes)
		}
		if enc.Encode(map[string]string{"type": "delta", "text": string(runes[i:end])}) != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	_ = enc.Encode(map[string]any{"type": "done", "source": source, "sourceTitle": title, "handoff": handoff})
	if flusher != nil {
		flusher.Flush()
	}
}
func (s *Service) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	var in RoomInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.EventID) < 1 || len(in.EventID) > 80 {
		fail(w, 422, "event_id_required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.roomReceipts[in.EventID]; ok {
		if previous.Input != in {
			fail(w, 409, "event_id_conflict")
			return
		}
		jsonResponse(w, 200, previous.Result)
		return
	}
	if in.Version != s.room.Version {
		fail(w, 409, "version_conflict")
		return
	}
	next := ""
	switch in.Action {
	case "reset":
		next = "idle"
	case "start":
		if s.room.State == "idle" {
			next = "live"
		}
	case "pause":
		if s.room.State == "live" {
			next = "paused"
		}
	case "resume":
		if s.room.State == "paused" {
			next = "live"
		}
	case "end":
		if s.room.State == "live" || s.room.State == "paused" {
			next = "ended"
		}
	}
	if next == "" {
		fail(w, 422, "invalid_transition")
		return
	}
	s.room = Room{next, s.room.Version + 1}
	s.roomReceipts[in.EventID] = RoomReceipt{in, s.room}
	jsonResponse(w, 200, s.room)
}
func (s *Service) AcceptEvent(w http.ResponseWriter, r *http.Request) {
	var in EventInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.ID) < 1 || len(in.ID) > 80 || len(in.Kind) < 1 || len(in.Kind) > 80 || len(in.Payload) > 1000 {
		fail(w, 422, "invalid_event")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.events[in.ID]; ok {
		if old != in {
			fail(w, 409, "event_id_conflict")
			return
		}
		jsonResponse(w, 200, map[string]any{"id": in.ID, "duplicate": true})
		return
	}
	s.attempts[in.ID]++
	if in.SimulateFailure && s.attempts[in.ID] == 1 {
		fail(w, 503, "simulated_transient_failure")
		return
	}
	s.events[in.ID] = in
	jsonResponse(w, 201, map[string]any{"id": in.ID, "duplicate": false, "attempts": s.attempts[in.ID]})
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Println("Local fixture service: 127.0.0.1:4738. No external AI, video, wallet or database connection.")
		return
	}
	service := &http.Server{Addr: "127.0.0.1:4738", Handler: NewService().Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = service.Shutdown(shutdown)
	}()
	log.Println("Local fixture backend: http://127.0.0.1:4738/demo-api/health")
	if err := service.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
