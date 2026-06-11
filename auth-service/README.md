# auth-service

Authentication, authorization, and audit service built in Go.

## Audit Log

Every authorization decision is recorded as a JSON event in `audit.log`:

```json
{"timestamp":"2026-06-10T12:00:00Z","user":"user123","role":"developer","action":"write","resource":"project","result":"allow"}
```

## Endpoints

| Method | Path | Auth | Required Permission | Description |
|---|---|---|---|---|
| GET | `/health` | No | — | Health check |
| GET | `/whoami` | Bearer JWT | — | Identity and role info |
| GET | `/projects` | Bearer JWT | read | List projects |
| POST | `/projects` | Bearer JWT | write | Create project |
| DELETE | `/projects` | Bearer JWT | delete | Delete project |

## Architecture

```
cmd/server/       — entry point
internal/auth/    — JWT parsing and validation
internal/middleware/ — auth middleware
internal/api/     — HTTP handlers
internal/models/  — shared types
internal/rbac/    — role-based authorization
internal/audit/   — structured audit logging (JSON, append-only)
tests/            — integration and benchmark tests
docs/             — architecture and incident docs
```
