# Interaction Workbench Demo

Runnable scenario demos for frontend, API and event workflows. Built with simulated data for reviewing implementation, architecture and delivery boundaries.

## First working workflow: event processing

Five JSON input events → field validation → duplicate filtering → Go API processing → bounded retry → structured receipts. Four unique events remain after filtering. A simulated transient failure exercises retry, and a second run safely replays already accepted event IDs.

### Run locally

Requirements: Go 1.23+ and Node.js 18.20+. This revision was tested with Go 1.23.3 and Node.js 18.20.1. No external packages or API keys are required for the API and CLI workflow.

Terminal 1, from the repository root:

```sh
npm run dev:api
```

Terminal 2:

```sh
npm test
npm run test:api
npm run demo:job
npm run demo:job
```

The fixture API listens only on `127.0.0.1:4738`. State is held in memory and clears when the service restarts. The CLI only permits its named local fixture endpoint.

## Included API modules

- `POST /demo-api/chat`: NDJSON conversation events, sample knowledge matching, persona fixture replies and human-handoff states.
- `GET /demo-api/room` and `POST /demo-api/room`: room lifecycle, expected-version checks and idempotent event replay.
- `POST /demo-api/events`: duplicate detection, conflicting-payload rejection and simulated transient failure.
- `GET /demo-api/health`: fixture mode and backend identity.

Tests exercise streamed events, input validation, lifecycle transitions, conflicts, concurrent deduplication, origin checks and input limits. This first revision exposes the API and CLI core; the React scenario views and real running screenshots are being integrated in this same repository.

## Implementation boundaries

Knowledge and character replies are fixtures. Live media, model APIs, wallets, chain transactions, production authentication and durable storage are integration options rather than active services. The sample never initiates a financial transaction. No customer code, private source data, credentials, user records or local caches are included.

## 中文说明

首个可运行流程为事件处理：读取5条模拟输入，去重为4条，通过 Go 接口处理，演示瞬时失败重试和再次执行的安全重放。可直接运行上方命令检查回执。Go 会话、房间和事件接口复用同一组自主实现的演示代码，React 视图随后在同一仓库接入。

**场景演示 Demo，使用模拟数据。**
