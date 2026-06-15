# 🗓️ Week 2 — User & Role Management

## 🎯 Objective
Add user management, password-based login, and persistent storage to the auth service.
Replace hardcoded users and roles with a real user store, a login API that returns JWTs, and an admin CRUD interface.
If you can log in, manage users, and persist state across restarts, the week is a win.

## 🔁 Weekly Loop (applied)

🧩 **Build** — User store, login endpoint, admin CRUD API
🔁 **Rebuild** — Swap in-memory store for SQLite without changing handlers
🐞 **Debug** — Privilege escalation: can a viewer create an admin user?
🚀 **Ship** — Push `auth-service` with persistent storage
👀 **Observe** — Measure in-memory vs SQLite latency on user operations

---

## ⚡ Energy-Based Plan

### ⚡ High Energy (Day 1–2)
**Focus: User Store & Login API**

Build a user management layer before worrying about persistence.
Implement:

- `User` struct:
```go
type User struct {
    ID           int64     `json:"id"`
    Username     string    `json:"username"`
    PasswordHash string    `json:"-"`
    Role         string    `json:"role"`
    CreatedAt    time.Time `json:"created_at"`
}
```

- `Store` interface:
```go
type Store interface {
    CreateUser(username, passwordHash, role string) (*User, error)
    GetUser(id int64) (*User, error)
    GetUserByUsername(username string) (*User, error)
    ListUsers() ([]*User, error)
    UpdateUser(id int64, username, passwordHash, role string) (*User, error)
    DeleteUser(id int64) error
}
```

- `InMemory` store with `sync.RWMutex`

**Endpoints:**

| Method | Path | Auth | Role | Description |
|--------|------|------|------|-------------|
| POST | `/auth/login` | No | — | Login, returns JWT |
| GET | `/admin/users` | JWT | admin | List all users |
| POST | `/admin/users` | JWT | admin | Create user |
| GET | `/admin/users/{id}` | JWT | admin | Get user by ID |
| PUT | `/admin/users/{id}` | JWT | admin | Update user |
| DELETE | `/admin/users/{id}` | JWT | admin | Delete user |

**Login flow:**
```
POST /auth/login
{"username": "admin", "password": "admin123"}
→ {"token": "eyJ..."}
```

Validation: bcrypt `CompareHashAndPassword`, missing fields → 400, bad creds → 401.

**Questions to answer as you build:**
- Why use `json:"-"` on `PasswordHash`?
- Why `sync.RWMutex` instead of `sync.Mutex`?
- Why pass `store.Store` as a closure to handlers rather than a global variable?
- Why does the admin mux get wrapped with both `AuthMiddleware` and `RequireRole`?
- What happens if the seed admin user already exists? (defensive coding)

**Deliverable:**
```
internal/models/user.go
internal/store/store.go
internal/api/auth.go      — LoginHandler
internal/api/admin.go     — List/Create/Get/Update/Delete handlers
internal/middleware/role.go  — RequireRole
cmd/server/main.go        — wire it up, seed admin
```

Commit: `feat: user store, login api, and admin crud`

**Learning Goal:**
password hashing (bcrypt), store abstraction, closure-based dependency injection in Go, Go 1.22+ route patterns (`{id}`).

---

### 🔧 Medium Energy (Day 3–4)
**Focus: SQLite Persistence**

Replace `InMemory` store with SQLite using `modernc.org/sqlite` (pure Go, no CGo).

- Add `modernc.org/sqlite` to go.mod
- Implement `SQLite` store behind the same `Store` interface
- Create table:
```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TEXT NOT NULL
);
```
- Swap `store.NewInMemory()` → `store.NewSQLite("users.db")` in `main.go`
- Verify data survives server restart

**Questions to answer:**
- How does the interface make this swap easy?
- What are the tradeoffs of `modernc.org/sqlite` vs `mattn/go-sqlite3`?
- Why no migration framework? When would you need one?

**Deliverable:**
Commit: `feat: sqlite user store`

**Learning Goal:**
interface-driven design for storage swapping, SQLite in Go, migration-free schema design for dev.

---

### 🔧 Medium Energy (Day 5)
**Focus: Hardening**

- Bcrypt cost factor via env var:
```go
cost, _ := strconv.Atoi(os.Getenv("BCRYPT_COST"))
if cost == 0 { cost = bcrypt.DefaultCost }
```

- Validate role on user create (reject unknown roles):
```go
validRoles := map[string]bool{"admin": true, "developer": true, "viewer": true}
if !validRoles[req.Role] { /* 400 */ }
```

- Rate limit on `/auth/login` (simple token bucket or `x/time/rate`):
```go
var loginLimiter = rate.NewLimiter(rate.Limit(5), 10) // 5 req/s, burst 10
if !loginLimiter.Allow() { /* 429 Too Many Requests */ }
```

**Questions to answer:**
- Why is rate limiting more important on `/auth/login` than on `/admin/users`?
- What does bcrypt's cost factor actually protect against? (rainbow tables? brute force? both?)
- Should the rate limiter be per-IP or global? What are the tradeoffs?

**Deliverable:**
Commit: `feat: hardening — bcrypt cost, role validation, rate limiting`

**Learning Goal:**
env var configuration, input validation as a security control, rate limiting for login endpoints.

---

### 🪫 Low Energy (Day 6)
**Focus: Security Ecosystem Familiarization**

Read one of:
- **OWASP Authentication Cheatsheet** — password storage, session management
- **bcrypt cost vs latency** — what cost factor is right for interactive vs batch auth?
- **Rate limiting strategies** — token bucket, leaky bucket, sliding window
- **SQLC** — type-safe SQL in Go (alternative to raw `database/sql`)

**Answer:**
- What threat does bcrypt's cost factor mitigate?
- At what request rate does a 5/s login rate limit start frustrating users?
- What attack does rate limiting protect against?

No contribution required.

---

### 😴 Day 7 — Full Reset
No coding unless you want to.
If curious:
- Read: `modernc.org/sqlite` docs
- Read: `x/time/rate` documentation
- Experiment: what happens to latency at bcrypt cost 14 vs cost 10?
Keep it lightweight.

---

## 🐞 Debug Task (non-negotiable)

**Privilege escalation test.**

Create a test scenario:
1. Login as `viewer` (or any non-admin)
2. Attempt `POST /admin/users` with the viewer's token

**Expected:** `403 Forbidden`
**Actual (if bug):** `200 OK` or the user is created

**Then:**
- Reproduce
- Diagnose
- Fix (if needed)
- Document

**Write:** `docs/incident-privilege-escalation.md`

Include:
- Symptoms
- Root cause
- Fix
- Verification
- Prevention

What if the bug is that `RequireRole` checks role against a hardcoded string but a future developer typos it? Could you write a test that catches that?

---

## 👀 Observation Task

Observe the full request lifecycle for user management.

Measure:
- Login latency (bcrypt verify + JWT generation)
- User CRUD latency (in-memory vs SQLite)
- Latency difference between allowed and denied admin requests

**Questions:**
- Where is most time spent? (bcrypt? SQLite? HTTP serialization?)
- How much overhead does `RequireRole` add vs `RequirePermission`?
- At what user count does the in-memory store become noticeably slower than SQLite?

---

## 🔬 Performance Investigation Lab

This week's investigation: **How does persistence affect auth latency?**

Benchmark:
- 10, 100, 1000 concurrent login requests
- 10, 100, 1000 concurrent user CRUD requests

Measure:
- Average latency
- P95 latency
- Error rate

Compare:
- In-memory store
- SQLite store (WAL mode)
- SQLite store (journal mode)

**Output:**
- Hypothesis
- Measurements
- Analysis
- Conclusion

Suggested tool: `hey`:
```bash
hey -n 100 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  http://localhost:8080/auth/login
```

---

## 🌍 Open Source Layer

Choose one:
- **SQLite** — `modernc.org/sqlite` (pure Go, CGo-free)
- **bcrypt** — `golang.org/x/crypto/bcrypt`
- **Rate limiting** — `golang.org/x/time/rate`

Read:
- The docs / source of your chosen library
- How it handles concurrency
- What edge cases it accounts for

**Answer:**
- What design patterns from these libraries could I apply to my own code?
- What security considerations did the library authors prioritize?

No contribution required yet.

---

## 🚀 Ship Requirement

By end of week:
- [ ] User model, Store interface, InMemory implementation
- [ ] `POST /auth/login` with bcrypt
- [ ] `GET/POST /admin/users`, `GET/PUT/DELETE /admin/users/{id}`
- [ ] `RequireRole("admin")` middleware
- [ ] SQLite store implementation
- [ ] Hardening: bcrypt cost config, role validation, rate limiting
- [ ] Privilege escalation investigation report
- [ ] Latency observation data
- [ ] Load test results
- [ ] Updated README
- [ ] All pushed to GitHub

No exceptions.

---

## 🎯 Week 2 Success Criteria

By the end, you should have:
- [ ] Replaced hardcoded users with a managed user store
- [ ] Implemented password-based login with bcrypt
- [ ] Built admin CRUD for user management
- [ ] Persisted state with SQLite
- [ ] Hardened the login endpoint (rate limiting, input validation)
- [ ] Investigated one privilege escalation failure scenario
- [ ] Measured persistence vs in-memory latency
- [ ] Load tested the login and admin endpoints
- [ ] Documented findings in `docs/`

The objective of Week 2 is not building production-grade user management. The objective is learning how identity management, persistence, and admin interfaces integrate with the authentication and authorization foundations from Week 1. Everything in Weeks 3+ depends on being able to manage who has access to what.
