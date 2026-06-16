# auth-service

Authentication, authorization, and audit service built in Go.

## Quickstart

```bash
# from repo root
make build-go      # or: cd auth-service && go build ./...
make test-go       # or: cd auth-service && go test ./...
make run           # start server on :8080
make token ROLE=admin   # generate a test JWT

# or just:
cd auth-service && go run ./cmd/server/
```

## Audit Log

Every authorization decision is recorded as a JSON event in `audit.log`:

```json
{"timestamp":"2026-06-10T12:00:00Z","user":"user123","role":"developer","action":"write","resource":"project","result":"allow"}
```

## Endpoints

| Method | Path | Auth | Required Permission | Description |
|---|---|---|---|---|---|
| POST | `/auth/login` | No | — | Authenticate, returns JWT |
| GET | `/health` | No | — | Health check |
| GET | `/whoami` | Bearer JWT | — | Identity and role info |
| GET | `/projects` | Bearer JWT | read | List projects |
| POST | `/projects` | Bearer JWT | write | Create project |
| DELETE | `/projects` | Bearer JWT | delete | Delete project |

## Seeded Users

On startup, the server seeds an admin user for testing:

| Username | Password | Role |
|---|---|---|
| `admin` | `admin123` | `admin` |

```bash
curl -X POST localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
# → {"token":"eyJ..."}
```

## Architecture

```
cmd/              — entry point
  server/         — main server
  token/          — JWT generator for testing
internal/
  auth/           — JWT parsing and validation
  middleware/     — auth middleware & RBAC enforcement
  api/            — HTTP handlers
  models/         — shared types (CustomClaims, ClaimsKey)
  rbac/           — role-based authorization
  audit/          — structured audit logging (JSON, append-only)
tests/            — integration and benchmark tests
docs/             — architecture and incident docs
```
