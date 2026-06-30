# Latency Observation

## Method

Each endpoint was hit 10 times in sequence. Times from `curl %{time_total}` were averaged.
Server: SQLite store, bcrypt cost 10, no artificial load.

## Results

| Operation | Avg Latency | Dominant Cost |
|---|---|---|
| LOGIN (`POST /auth/login`) | **102.2 ms** | bcrypt.CompareHashAndPassword |
| CREATE USER (`POST /admin/users`) | **131.9 ms** | bcrypt.GenerateFromPassword |
| LIST USERS (`GET /admin/users`) | **0.9 ms** | SQLite SELECT (full scan) |
| GET USER (`GET /admin/users/1`) | **0.8 ms** | SQLite SELECT by PK |
| UPDATE USER (`PUT /admin/users/1`) | **16.5 ms** | SQLite SELECT + UPDATE + SELECT |
| DELETE USER (`DELETE /admin/users/N`) | **13.1 ms** | SQLite DELETE |
| DENIED (developer → `GET /admin/users`) | **0.6 ms** | JWT validate + role check (no DB) |

## Analysis

### Bcrypt dominates write latency
- `CreateUser` (131.9 ms) and `Login` (102.2 ms) are ~100× slower than read operations.
- Bcrypt cost 10 (`~10ms` per hash) accounts for nearly all of that time. SQLite INSERT/UPDATE adds ~1–2 ms.
- At cost 14 (Ticket 10), these would rise to ~1600 ms (cost 14 ≈ 16× slower than cost 10).

### Read operations are sub-millisecond
- `ListUsers` (0.9 ms) and `GetUser` (0.8 ms) are dominated by JSON serialization overhead, not SQLite.
- SQLite on the local filesystem is effectively zero-latency for these workloads.

### Denied requests are cheapest
- A non-admin hitting an admin endpoint (0.6 ms) is faster than a read because `RequireRole` short-circuits before `adminMux` is reached — no DB query at all.

### MIDDLEWARE OVERHEAD IS NEGLIGIBLE
- JWT parsing + role comparison adds <0.5 ms per request.

## Recommendations

1. **Keep bcrypt cost at 10 for now** — cost 14 would push create/login past 1.5 s, which is noticeable in interactive use.
2. **If login latency becomes a problem**, consider caching bcrypt results (short-lived) or using a faster KDF (e.g., Argon2id with configurable params).
3. **No observable difference** between allowed and denied request paths — the middleware chain adds no meaningful overhead.
