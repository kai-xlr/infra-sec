# 🗓️ Week 3 — Persistence, Hardening & Production Readiness

## 🎯 Objective
Replace the in-memory store with SQLite, harden the auth endpoints, and add production-readiness features (graceful shutdown, structured errors, request logging).

## 📦 Tickets

### Tier 1: SQLite Persistence (Tickets 1–9)

| # | Ticket | File(s) | Behavior |
|---|---|---|---|
| 1 | Add `modernc.org/sqlite` dependency | `go.mod`, `go.sum` | `go get modernc.org/sqlite`, verify build |
| 2 | `SQLiteStore` struct + constructor | `internal/store/sqlite.go` | Opens/creates DB file, runs `CREATE TABLE IF NOT EXISTS users` |
| 3 | `SQLiteStore.CreateUser` | `internal/store/sqlite.go` | INSERT with duplicate username check → return error |
| 4 | `SQLiteStore.GetUser` / `GetUserByUsername` | `internal/store/sqlite.go` | SELECT by id / username → error if not found |
| 5 | `SQLiteStore.ListUsers` | `internal/store/sqlite.go` | SELECT all → empty slice if none |
| 6 | `SQLiteStore.UpdateUser` | `internal/store/sqlite.go` | UPDATE by id, unique check → error |
| 7 | `SQLiteStore.DeleteUser` | `internal/store/sqlite.go` | DELETE by id → error if not found |
| 8 | Swap `NewInMemoryStore` → `NewSQLiteStore` in main.go | `cmd/server/main.go` | Replace in-memory init with SQLite init |
| 9 | Verify data survives restart | Manual test | Start server, create user, restart, GET user → 200 |

### Tier 2: Hardening (Tickets 10–12)

| # | Ticket | File(s) | Behavior |
|---|---|---|---|
| 10 | Bcrypt cost via `BCRYPT_COST` env var | `internal/api/auth.go`, `internal/api/admin.go` | Default to `bcrypt.DefaultCost`, parse from env |
| 11 | Validate role on user create | `internal/api/admin.go` | Reject unknown roles → 400 |
| 12 | Rate limit `/auth/login` | `internal/api/auth.go` | 5 req/s burst 10 → 429 |

### Tier 3: Debug & Observe (Tickets 13–15)

| # | Ticket | File(s) | Behavior |
|---|---|---|---|
| 13 | Privilege escalation investigation | `docs/incident-privilege-escalation.md` | Create viewer, attempt POST /admin/users → 403 |
| 14 | Latency observation | `docs/latency-observation.md` | Measure login + CRUD latency (in-memory vs SQLite) |
| 15 | Load test with `hey` | `docs/load-test-results.md` | 100/1000 concurrent requests, report P50/P95/error rate |

### Tier 4: Production Readiness (Tickets 16–18)

| # | Ticket | File(s) | Behavior |
|---|---|---|---|
| 16 | Graceful shutdown | `cmd/server/main.go` | `signal.NotifyContext` + `http.Server.Shutdown` |
| 17 | JSON error responses | `internal/api/`, `internal/middleware/` | Replace `http.Error` with `{"error": "msg"}` |
| 18 | Request logging middleware | `internal/middleware/logger.go` | Log method, path, status, duration |

---

## 🚀 Shipping Criteria

By end of week:
- [ ] SQLite store behind the `Store` interface
- [ ] Data survives restart
- [ ] Bcrypt cost configurable via env var
- [ ] Role validation on create
- [ ] Rate limiting on login
- [ ] Graceful shutdown
- [ ] JSON error responses
- [ ] Request logging middleware
- [ ] Privilege escalation report
- [ ] Latency observation report
- [ ] Load test report
