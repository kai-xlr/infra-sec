# auth-service

Authentication, authorization, and audit service built in Go.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | No | Health check |
| GET | `/whoami` | Bearer JWT | Identity and role info |

## Architecture

```
cmd/server/       — entry point
internal/auth/    — JWT parsing and validation
internal/middleware/ — auth middleware
internal/api/     — HTTP handlers
internal/models/  — shared types
internal/rbac/    — role-based authorization (WIP)
internal/audit/   — structured audit logging (WIP)
tests/            — integration and benchmark tests
docs/             — architecture and incident docs
```
