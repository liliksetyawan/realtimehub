# RealtimeHub

> 50 000-connection WebSocket fan-out for in-app notifications. Built on `lesismal/nbio` netpoll, sharded hub, Redis pub-sub for cross-node fan-out, with sequence-based recovery so reconnecting clients catch up cleanly.

This is a from-scratch implementation of the C50K-class problem in Go: hold tens of thousands of WebSocket connections per node stably, then fan a notification out to the right ones in milliseconds.

A polished React SPA on top demonstrates the full UX: login, real-time bell with unread count, in-app toasts, and an admin panel to push notifications.

> **Status**: scaffold landed, building Phase 1 (WebSocket server + hub).

---

## Why this exists

Most "real-time demo" repos pick `gorilla/websocket`, run a chat in two browser tabs, and call it a day. That's fine for tutorials — it falls over at scale. At 50 000 connections, the goroutine-per-connection model uses ~800 MB of stack alone, GC churn climbs, and the single-node read/write loop becomes the bottleneck.

RealtimeHub is the *production-shaped* version of that problem: epoll-based netpoll, sharded hub, bounded write buffers, kernel tuning, and a documented load test that proves the number.

---

## Stack

- **Go 1.25** — server
- **lesismal/nbio** — epoll-based WebSocket library (~4-8 KB/conn vs 16+ for goroutine-per-conn)
- **PostgreSQL** — notification persistence (history + replay-on-reconnect source)
- **Redis** (rueidis client) — cross-node pub-sub fan-out
- **JWT (HS256)** — auth handed off at WebSocket handshake
- **React 18 + TS + Vite + Redux Toolkit** — SPA (login / dashboard / admin)
- **k6** — load testing the 50k claim
- **Docker Compose** — infra

---

## Roadmap

- [x] Phase 0 · scaffold
- [ ] Phase 1 · nbio server + sharded hub + ping/pong
- [ ] Phase 2 · JWT handshake auth + login endpoint
- [ ] Phase 3 · notification persistence + admin REST API
- [ ] Phase 4 · multi-node fan-out via Redis pub-sub
- [ ] Phase 5 · reconnect + sequence recovery
- [ ] Phase 6 · React SPA (login, dashboard, admin)
- [ ] Phase 7 · k6 ramp-to-50k + kernel tuning + benchmark numbers
- [ ] Phase 8 · CI + polished README

---

## License

MIT
