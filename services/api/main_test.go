package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func req(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestChat(t *testing.T) {
	h := NewService().Handler()
	for _, v := range []struct {
		body, contains string
		code           int
	}{{`{"message":"订单状态","locale":"zh"}`, "guide-order", 200}, {`{"message":"unknown topic","locale":"en"}`, `"handoff":true`, 200}, {`{"message":"hi","locale":"en","persona":"librarian"}`, "Hoshino", 200}, {`{"message":"","locale":"en"}`, "message_length", 422}, {`{"message":"hi","locale":"xx"}`, "locale_required", 422}, {`{"message":"hi","locale":"en","persona":"missing"}`, "unknown_persona", 422}, {`{"message":"hi","locale":"en","extra":1}`, "invalid_json", 400}, {`{} {}`, "trailing_json", 400}} {
		w := req(h, "POST", "/demo-api/chat", v.body)
		if w.Code != v.code || !strings.Contains(w.Body.String(), v.contains) {
			t.Fatalf("%s: %d %s", v.body, w.Code, w.Body.String())
		}
		if w.Code == 200 {
			lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
			if len(lines) < 3 {
				t.Fatal("expected streamed events")
			}
			for _, line := range lines {
				var x any
				if json.Unmarshal([]byte(line), &x) != nil {
					t.Fatal("invalid stream json")
				}
			}
		}
	}
}
func TestRoomTransitionsAndReplay(t *testing.T) {
	h := NewService().Handler()
	body := `{"action":"start","expectedVersion":1,"eventId":"e1"}`
	a := req(h, "POST", "/demo-api/room", body)
	b := req(h, "POST", "/demo-api/room", body)
	if a.Code != 200 || a.Body.String() != b.Body.String() {
		t.Fatal("same event must replay same result")
	}
	for _, v := range []struct {
		body string
		code int
	}{{`{"action":"pause","expectedVersion":1,"eventId":"e2"}`, 409}, {`{"action":"end","expectedVersion":1,"eventId":"e1"}`, 409}, {`{"action":"pause","expectedVersion":2,"eventId":"e2"}`, 200}, {`{"action":"start","expectedVersion":3,"eventId":"e3"}`, 422}, {`{"action":"end","expectedVersion":3,"eventId":"e4"}`, 200}} {
		if w := req(h, "POST", "/demo-api/room", v.body); w.Code != v.code {
			t.Fatalf("%s: %d", v.body, w.Code)
		}
	}
}
func TestEventRetryConflictAndConcurrentDedup(t *testing.T) {
	h := NewService().Handler()
	body := `{"id":"fixture","kind":"sample","payload":"x","simulateFailure":true}`
	if w := req(h, "POST", "/demo-api/events", body); w.Code != 503 {
		t.Fatal(w.Code)
	}
	if w := req(h, "POST", "/demo-api/events", body); w.Code != 201 {
		t.Fatal(w.Code)
	}
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			w := req(h, "POST", "/demo-api/events", body)
			if w.Code != 200 {
				t.Errorf("duplicate %d", w.Code)
			}
		}()
	}
	group.Wait()
	if w := req(h, "POST", "/demo-api/events", strings.Replace(body, `"x"`, `"y"`, 1)); w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func TestOriginBodyAndMethod(t *testing.T) {
	h := NewService().Handler()
	r := httptest.NewRequest("POST", "/demo-api/chat", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://unrelated.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := req(h, "GET", "/demo-api/chat", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w := req(h, "POST", "/demo-api/chat", `{"message":"`+strings.Repeat("x", 5000)+`","locale":"en"}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
