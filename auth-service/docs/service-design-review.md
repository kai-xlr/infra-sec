# Service Design Review — auth-service

## 1. Request Lifecycle

An HTTP request travels through the following layers in order:

```
Client ──▶ net/http Server ──▶ RequestLogger ──▶ Router
                                                    │
                                          ┌─────────┴──────────┐
                                          ▼                    ▼
                                   Public routes        Protected routes
                                   (no middleware)       │
                                                         ▼
                                                   AuthMiddleware
                                                   (JWT validation)
                                                         │
                                                         ▼
                                              [RequireRole | RequirePermission]
                                                   (RBAC check)
                                                         │
                                                         ▼
                                                     Handler
                                                    (JSON I/O)
                                                         │
                                                         ▼
                                                     Store
                                           (InMemory / SQLite)
                                                         │
                                                         ▼
                                              JSON Response ◀────
```

**End-to-end trace for `GET /admin/users` (admin token):**

1. `net/http` server on `:8080` accepts connection, parses HTTP request
2. `RequestLogger` captures start time, wraps `ResponseWriter`
3. Router matches `/admin/` prefix → routes to `adminMux` (wrapped in `AuthMiddleware` + `RequireRole("admin")`)
4. `AuthMiddleware` extracts `Authorization: Bearer <token>`, calls `token.ValidateToken`
   - Parses JWT with HS256, checks signature, expiry, issuer
   - Injects `*model.CustomClaims{Role: "admin", Username: "..."}` into `context.Context`
5. `RequireRole("admin")` reads claims from context, checks `claims.Role == "admin"`
   - Logs audit event: `{"action": "ACCESS_ROLE_admin", "result": "allow"}`
6. `ListUsersHandler` handles the GET, calls `store.ListUsers("")`
7. `SQLiteStore.ListUsers` acquires `RLock`, runs `SELECT * FROM users`, parses rows to `[]*model.User`, releases `RLock`
8. Handler marshals to JSON, writes `200 OK` with `Content-Type: application/json`
9. `RequestLogger` logs `"GET /admin/users 200 1.2ms"`

**Error path (viewer token on same route):**

Steps 1–4 same. Step 5: `RequireRole("admin")` sees `role: "viewer"`, logs `{"result": "deny"}`, returns `403 Forbidden` via `jsonError`. No DB query occurs.

## 2. Middleware Dependency Graph

```
RequestLogger (outermost, wraps entire mux)
 │
 └── /health ──▶ HealthHandler
 │
 └── POST /auth/login ──▶ rate.Limiter.Allow()? ──▶ LoginHandler
 │                           │
 │                       429 on exhaust
 │
 └── Protected routes (any path with auth requirements)
      │
      └── AuthMiddleware (injects claims into context)
           │
           ├── RequireRole("admin") ──▶ adminMux
           │    │                         ├── ListUsersHandler  (GET  /admin/users)
           │    │                         ├── CreateUserHandler (POST /admin/users)
           │    │                         ├── GetUserHandler    (GET  /admin/users/{id})
           │    │                         ├── UpdateUserHandler (PUT  /admin/users/{id})
           │    │                         └── DeleteUserHandler (DELETE /admin/users/{id})
           │
           ├── WhoamiHandler    (GET /whoami)
           │
           ├── authHandler.MeHandler           (GET /me)
           ├── authHandler.PasswordChangeHandler (POST /auth/password)
           │
           └── RequirePermission(action, resource)
                └── ProjectsHandler (GET/POST/DELETE /projects)
```

**Dependency rules:**

- `AuthMiddleware` must run before `RequireRole` or `RequirePermission` — both depend on `CustomClaims` being present in context
- `RequireRole` and `RequirePermission` are mutually exclusive by route (no route uses both)
- `RequestLogger` is outermost so it captures the final status code and duration for every request including auth failures
- Rate limiter is applied directly in `LoginHandler` via a package-level `rate.Limiter`, not as middleware — this means it runs after routing but before handler logic

**Injections:**

| Middleware | Injects | Consumed by |
|---|---|---|
| `RequestLogger` | `*responseWriter` (wraps `http.ResponseWriter`) | Its own deferred log |
| `AuthMiddleware` | `*model.CustomClaims` via `context.WithValue(ctx, ClaimsKey, claims)` | `RequireRole`, `RequirePermission`, `MeHandler`, `WhoamiHandler`, `PasswordChangeHandler` |

## 3. Error Handling Strategy

**Pattern:** Error responses use a `jsonError` helper defined (duplicated) in both `handler/errors.go` and `middleware/errors.go`:

```go
func jsonError(w http.ResponseWriter, msg string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

**Error matching:** Store errors are matched via `strings.Contains(err.Error(), keyword)`. No custom error types or sentinel errors.

| Keyword | Mapped Status | Locations |
|---|---|---|
| `"not found"` | 404 | `GetUser`, `UpdateUser`, `DeleteUser` handlers |
| `"already exists"` | 409 | `CreateUser` handler, seed admin in main.go |
| `"already taken"` | 409 | `UpdateUser` handler |

**Response codes by endpoint:**

| Code | Meaning | Endpoints |
|---|---|---|
| 200 | Success | All read/write handlers |
| 201 | Created | `CreateUser` |
| 204 | No Content | `DeleteUser` |
| 400 | Bad Request (validation) | `Login`, `CreateUser`, `PasswordChange` — missing fields, invalid JSON, bad Content-Type, empty passwords |
| 401 | Unauthorized | `AuthMiddleware` (missing/expired/invalid token), `LoginHandler` (bad credentials) |
| 403 | Forbidden | `RequireRole` / `RequirePermission` (insufficient permissions) |
| 404 | Not Found | `GetUser`, `UpdateUser`, `DeleteUser` (nonexistent ID) |
| 409 | Conflict | `CreateUser` (duplicate username), `UpdateUser` (username taken) |
| 429 | Too Many Requests | `LoginHandler` (rate limit exhausted) |
| 405 | Method Not Allowed | Any handler when `r.Method` does not match (explicit check, no `http.MethodNotAllowedHandler`) |
| 500 | Internal Server Error | Store failures, JSON encoding failures |

**Notable design decisions:**

- Login returns a generic "invalid credentials" message regardless of whether the username exists or the password is wrong — prevents user enumeration
- No structured error types — the `strings.Contains` pattern is fragile across driver versions and does not compose well
- Request-scoped validation (missing fields, empty strings) is handled inline per handler rather than through a shared validation layer

## 4. Storage Layer

**Interface (`internal/store/store.go`):**

```go
type Store interface {
    CreateUser(username, passwordHash, role string) (*model.User, error)
    GetUser(id int64) (*model.User, error)
    GetUserByUsername(username string) (*model.User, error)
    ListUsers(role string) ([]*model.User, error)
    UpdateUser(id int64, username, passwordHash, role string) (*model.User, error)
    DeleteUser(id int64) error
}
```

**Two implementations:**

| Aspect | InMemoryStore | SQLiteStore |
|---|---|---|
| Backing | `map[int64]*model.User` + `sync.RWMutex` | `modernc.org/sqlite` (pure Go, no CGo) |
| Persistence | None (lost on restart) | File-based (`users.db`) |
| Concurrency | RWMutex: reads parallel, writes serial | RWMutex + SQLite internal serialization |
| `GetUserByUsername` | O(n) linear scan | O(log n) index scan (UNIQUE on username) |
| `ListUsers(role)` | O(n) with optional filter | `SELECT * FROM users WHERE role = ?` |
| Test usage | All existing tests | Not used in tests |
| Startup cost | None | `CREATE TABLE IF NOT EXISTS` on every start |

**Migration strategy:** There is no formal migration system. The schema is defined in `createTables()` called from `NewSQLiteStore`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT ''
);
```

**Schema oddities:**
- `created_at` and `updated_at` are stored as `TEXT` (ISO 8601 / RFC 3339), parsed with `time.Parse(time.RFC3339, ...)` on read
- No `NOT NULL DEFAULT ''` on `updated_at` in the create — it's set explicitly on INSERT
- No foreign keys, no indexes beyond the implicit `UNIQUE` on `username` and the `PRIMARY KEY` on `id`
- Timestamps are server-local — no timezone normalization enforced

**What's NOT supported:**
- Transactional rollback across handler+store boundaries
- Pagination (`ListUsers` returns all matching rows)
- Partial updates via the interface (handlers work around this with `*string` types)
- Connection pooling configuration (single `*sql.DB` with defaults)

## 5. Security Architecture

### JWT Authentication

| Property | Value |
|---|---|
| Algorithm | HMAC-SHA256 (HS256) |
| Library | `github.com/golang-jwt/jwt/v5` |
| Secret source | `JWT_SECRET` env var; fallback `"your-highly-secure-secret-key-change-in-production"` |
| Expiry | 15 minutes from issuance |
| Issuer | `"auth-service"` (validated on parse) |
| Custom claims | `role` (string), `username` (string) |

**Validation checks in `ValidateToken`:**
- Signature verification (secret must match)
- Algorithm check (rejects `none` algorithm — jwt library default rejects unverified tokens)
- Expiration (`ExpiresAt` must be in the past)
- Issuer must be `"auth-service"`

**Notable: `cmd/token/main.go` does NOT read `JWT_SECRET`** — it hardcodes the same fallback string. This means tokens generated by the CLI use the default secret even when the server is configured with a custom `JWT_SECRET`.

### Password Hashing

- Algorithm: bcrypt via `golang.org/x/crypto/bcrypt`
- Cost: `BCRYPT_COST` env var or `bcrypt.DefaultCost` (10)
- Comparison: `bcrypt.CompareHashAndPassword` on login (≈10ms at cost 10)
- Hashing: `bcrypt.GenerateFromPassword` on create/password-change (≈10ms at cost 10)

### Rate Limiting

- Scope: `POST /auth/login` only
- Algorithm: Token bucket (`x/time/rate`, single `rate.NewLimiter(5, 10)`)
- Rate: 5 tokens/second, burst 10
- Granularity: Global (single bucket for all clients) — not per-IP or per-user

### Authorization

| Mechanism | Enforcement | Effect |
|---|---|---|
| `RequireRole(role)` | Middleware | Checks `claims.Role == role` (exact string match) |
| `RequirePermission(action, resource)` | Middleware | Calls `rbac.Authorize(role, action)` against `PolicyMatrix` |

**RBAC Matrix (static, compile-time):**

| Role | Permissions |
|---|---|
| admin | read, write, delete |
| developer | read, write |
| viewer | read |

**No role hierarchy** — a `RequireRole("admin")` check does NOT match a developer. The Rust policy-engine supports role hierarchy; the Go auth-service does not.

### Audit Logging

- Append-only JSON file (`audit.log`)
- Mutex-protected writes via `sync.Mutex`
- Each event records: `timestamp`, `user`, `role`, `action`, `resource`, `result` (allow/deny)
- Written by `RequireRole` and `RequirePermission` middleware on every authorization decision

## 6. Restart Behavior

| Component | Behavior on Restart |
|---|---|
| **SQLite database** | Survives restart. File `users.db` is opened, tables verified via `CREATE TABLE IF NOT EXISTS`. |
| **Seed admin user** | Created on every startup if `admin` user does not exist (idempotent — `"already exists"` error is silently ignored). |
| **Rate limiter** | Resets. `rate.NewLimiter(5, 10)` starts fresh — no memory of prior requests. |
| **Audit log** | Opens in append mode (`O_APPEND`). Prior entries are preserved; new entries appended. |
| **JWT signing** | Same `JWT_SECRET` env var → existing tokens remain valid until their 15-minute expiry. |
| **Cleanup worker** | Restarts with `CLEANUP_INTERVAL` timer from zero. |
| **In-memory state** | `GetUserByUsername` linear scan, `nextID` counter — all rebuilt from SQLite on first access. |

**Seed user details:**
- Username: `"admin"`
- Password: `"admin123"` (bcrypt-hashed with cost 10)
- Role: `"admin"`
- If the SQLite file is deleted between restarts, the seed user is recreated with the same credentials.

## 7. Failure Modes

### SQLite Database Corruption
- **Cause:** Disk full, unclean shutdown (SIGKILL), filesystem error
- **Detection:** `sql.Open` succeeds (no integrity check), first query fails with opaque error from `modernc.org/sqlite`
- **Impact:** Service starts but all store operations return 500. Health check still returns `200 OK` (it has no DB dependency).
- **Mitigation:** None. No integrity check on startup, no backup/restore mechanism, no read-only fallback.

### Bcrypt Cost Slowdown
- **Cause:** `BCRYPT_COST` set to 14+ (≈160ms per hash). Multiple concurrent login/create requests amplify.
- **Impact:** Login latency spikes from 100ms to >1.5s. Golang HTTP server goroutine-per-request model means goroutines pile up, memory grows, eventually OOM.
- **Mitigation:** None enforced. The env var is accepted without upper-bound validation.

### Rate Limiter Exhaustion
- **Cause:** 6+ login requests within 1 second — all subsequent requests get 429 for ~1 second
- **Impact:** Legitimate users briefly locked out. No per-IP isolation means one aggressive client can DoS all others.
- **Mitigation:** Single global bucket offers no isolation. No queue or backoff.

### JWT Secret Rotation
- **Cause:** Operator changes `JWT_SECRET` while server is running, then restarts
- **Impact:** All existing tokens immediately invalid. Every client gets 401 until they re-login.
- **Mitigation:** The service uses a single shared secret with no grace period or multi-key support.

### CLI Token Mismatch
- **Cause:** `cmd/token/main.go` ignores `JWT_SECRET` env var. Operator sets a custom secret for the server but generates tokens with the CLI.
- **Impact:** All CLI-generated tokens are rejected by the server with 401. Hard to debug because both commands appear to accept `JWT_SECRET`.
- **Mitigation:** The CLI tool should either read the env var or at minimum print a warning about the hardcoded secret.

### SQLite Concurrent Write Contention
- **Cause:** Multiple concurrent `CreateUser` or `UpdateUser` calls
- **Impact:** The `RWMutex` serializes all write operations, but SQLite itself is single-writer. Under load, writes queue behind the mutex and then again behind SQLite's internal lock.
- **Mitigation:** The double-locking (application mutex + SQLite lock) adds overhead without benefit. Reads are blocked during writes despite SQLite supporting concurrent reads during writes (WAL mode — not enabled here).

## 8. Tradeoffs

### SQLite vs PostgreSQL

| Criterion | SQLite (chosen) | PostgreSQL |
|---|---|---|
| Operational complexity | Zero — single file, no daemon | Requires server, auth, connection pooling, backups |
| Concurrent writes | Single writer (≈60-100 writes/sec) | Multi-writer with MVCC (thousands/sec) |
| Data safety | `PRAGMA synchronous=FULL` available but not configured | Durable by default with WAL + fsync |
| Backup | `cp users.db users.db.bak` (may be inconsistent without `.backup` API) | `pg_dump`, streaming replication, point-in-time recovery |
| Schema migrations | Manual `ALTER TABLE` applied on startup via `IF NOT EXISTS` | Dedicated migration tools (golang-migrate, goose) |
| Cross-platform | Pure Go via `modernc.org/sqlite`, cross-compiles anywhere | Works everywhere but C Go driver requires CGo |

**Verdict:** SQLite is correct for a single-node control-plane service with low write volume. If the service ever needs horizontal scaling (multiple auth-service instances), SQLite becomes a bottleneck and Postgres is required.

### Pure-Go SQLite (`modernc.org/sqlite`) vs CGo (`mattn/go-sqlite3`)

| Criterion | modernc.org/sqlite (chosen) | mattn/go-sqlite3 |
|---|---|---|
| Build complexity | Zero — no C toolchain needed | Requires gcc, `CGO_ENABLED=1`, cross-compilation pain |
| Performance | ~10-15% slower (transpiled C → Go, no compiler optimizations) | Native C speed |
| Memory | Higher (Go translation layer) | Lower (direct C calls) |
| Compatibility | Can build with `CGO_ENABLED=0` | Requires CGo |
| SQLite version | Tracks SQLite 3.43 (at time of dependency pin) | Tracks SQLite via amalgamation |

**Verdict:** `modernc.org/sqlite` is the right choice for this project — the performance difference is negligible at auth-service scale, and the elimination of CGo build complexity is a significant DX win. Revisit if SQLite-level performance tuning (custom `PRAGMA`, advanced features) becomes necessary.

### `x/time/rate` (Token Bucket) vs Sliding Window Log

| Criterion | Token Bucket (chosen) | Sliding Window Log |
|---|---|---|
| Memory | O(1) — single counter + timestamp | O(n) — per-request timestamp log |
| Burst handling | Natural — bucket depth allows bursts | Requires separate burst configuration |
| Per-IP accounting | Not built-in — single bucket only | Natural — per-IP key in window map |
| Implementation | One `rate.NewLimiter` call | Manual map + slice pruning |
| Reset behavior | Refills continuously at rate | Window slides continuously |

**Verdict:** `x/time/rate` is simple and correct for the current single-bucket use case. However, the lack of per-IP isolation is a real weakness — a sliding window with per-IP keys would be strictly better for brute-force protection. The current approach trades robustness for simplicity.

### Static (Compile-Time) vs Dynamic RBAC

| Criterion | Static (chosen) | Dynamic |
|---|---|---|
| Safety | Type-checked, impossible to have undefined roles/permissions at runtime | Requires validation layer; config errors cause auth bypass |
| Deploy flexibility | Requires code change + rebuild to modify policy | Hot-reloadable via config file or API |
| Audit | Policy is in git — full change history | Requires separate audit trail for config changes |
| Complexity | Trivial — one map + one function | Requires config parsing, validation, caching, reloading |

**Verdict:** Static is correct for Phase 1. Dynamic RBAC (potentially backed by the Rust policy-engine) should be added when the auth service needs to support customer-defined roles or on-the-fly policy changes.

## 9. AI Review Challenges

### Challenge 1: Hardcoded JWT secret fallback is not safe even for development

**Assumption:** "The hardcoded fallback `your-highly-secure-secret-key-change-in-production` is acceptable because it's clearly a placeholder and will be changed before production."

**Challenge:** This assumption is false on three levels:
1. The `cmd/token/main.go` CLI tool duplicates the hardcoded secret independently — it does not read `JWT_SECRET`. A developer who sets `JWT_SECRET=prod-secret` and uses the CLI to generate tokens will get tokens signed with the fallback secret, which the server will reject. This creates a confusing, hard-to-debug failure.
2. The fallback should fail hard (panic or refuse to start) rather than silently proceed with a known-default secret. Multiple real-world breaches (e.g., Uber 2016, various IoT device exploits) trace back to unchanged default credentials.
3. The secret appears in both `token/token.go` and `cmd/token/main.go` — this is a maintenance hazard. A developer changing one without the other introduces a silent mismatch.

**Recommendation:** Remove the fallback. Make `JWT_SECRET` mandatory — the server should refuse to start if it's unset. Consolidate secret loading into a single shared location rather than duplicating across packages.

### Challenge 2: A global rate limiter on login provides weak brute-force protection

**Assumption:** "Rate limiting the login endpoint at 5 requests/second with a single token bucket is sufficient to prevent credential stuffing."

**Challenge:** A single global bucket means:
- A slow, single-IP attacker is rate-limited (good)
- A distributed attacker (botnet, 10+ IPs) gets 5 requests/second per actor — 10 IPs = 50 requests/second total, which is well within brute-force territory
- A single aggressive client can exhaust the bucket for all other clients (no isolation)
- The login endpoint has no account lockout, no progressive delay, and no anomaly detection

The 5 requests/second limit also interacts poorly with legitimate retry patterns. A mobile client that sends 3 rapid retries after a network blip burns 60% of the global budget.

**Recommendation:** Implement per-IP rate limiting using a sliding window or the existing token bucket keyed by client IP. Add per-account progressive delay after N failures.

### Challenge 3: `strings.Contains` error matching is not a robust error handling strategy

**Assumption:** "Checking store errors with `strings.Contains(err.Error(), "not found")` is equivalent to typed error handling."

**Challenge:** Error messages are implementation details, not API contracts:
- `modernc.org/sqlite` error strings could change between versions
- A user whose username contains the substring `"not found"` would cause a false positive if `GetUserByUsername("notfound_user")` is implemented
- The pattern does not compose — you cannot wrap errors with additional context without breaking the string match
- SQLite error messages differ from InMemoryStore error messages ("record not found" vs "user not found"), making the interface contract implicit and fragile

Of the six `Store` interface methods, five return `error` with no documented error contract. Callers are forced to reverse-engineer error strings from each implementation.

**Recommendation:** Define sentinel errors in the `store` package:
```go
var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")
```
Use `errors.Is(err, ErrNotFound)` in handlers. This is the standard Go pattern and eliminates the fragility.
