# Architecture — auth-service

## Overview

The auth-service is a Go HTTP server that provides authentication, authorization, and audit logging for the infra-sec platform. It uses JWT Bearer tokens for authentication and a role-based access control (RBAC) matrix for authorization.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────┐
│  HTTP Request                                       │
│  ┌──────────┐   ┌───────────────┐   ┌────────────┐ │
│  │  Client  │──▶│  AuthMiddleware│──▶│   Router   │ │
│  └──────────┘   └───────┬───────┘   └─────┬──────┘ │
│                         │                  │        │
│                         ▼                  ▼        │
│                   ┌──────────┐     ┌──────────────┐ │
│                   │  auth.   │     │  RequirePerm │ │
│                   │Validate  │     │  Middleware  │ │
│                   │ Token    │     │  (per route) │ │
│                   └──────────┘     └──────┬───────┘ │
│                                           │         │
│                    ┌──────────────────────┘         │
│                    ▼                                │
│              ┌────────────┐                         │
│              │  handlers  │                         │
│              │  (api.)    │                         │
│              └─────┬──────┘                         │
│                    │                                │
│                    ▼                                │
│              ┌────────────┐            ┌──────────┐ │
│              │   RBAC     │            │  Audit   │ │
│              │  Engine    │◀──────────▶│  Logger  │ │
│              └────────────┘            └──────────┘ │
└─────────────────────────────────────────────────────┘
```

## Layers

### 1. Auth Middleware (`internal/middleware/middleware.go`)
- Extracts the Bearer token from the `Authorization` header
- Calls `auth.ValidateToken()` to parse and verify the JWT
- Injects `*models.CustomClaims` into `context.Context` via `ClaimsKey`
- Forwards to the next handler on success; returns `401 Unauthorized` on failure

### 2. Token Validation (`internal/auth/auth.go`)
- Parses the JWT using `golang-jwt/jwt/v5` with HMAC-SHA256
- Validations performed:
  - **Signature**: HMAC secret must match
  - **Algorithm**: only HMAC (HS256) accepted — `unexpected signing method` otherwise
  - **Issuer**: must be `auth-service`
  - **Expiration**: must be present and in the future
- Returns `*models.CustomClaims{Role string}` on success

### 3. RBAC Engine (`internal/rbac/engine.go`)
- Authorize(role, action) performs a lookup against `PolicyMatrix`
- Admin: read, write, delete
- Developer: read, write
- Viewer: read
- Returns `true`/`false`

### 4. Permission Middleware (`internal/middleware/authorize.go`)
- Factory function `RequirePermission(action, resource, *audit.Logger)` returns `func(http.Handler) http.Handler`
- Extracts `CustomClaims` from context, calls `rbac.Authorize(role, action)`
- On deny: writes `403 Forbidden`, logs an audit event with `"deny"` result
- On allow: logs an audit event with `"allow"` result, forwards to the next handler

### 5. Audit Logger (`internal/audit/audit.go`)
- Concurrency-safe, append-only JSON logger
- Writes to `audit.log` (configurable path)
- Each event: `timestamp`, `user`, `role`, `action`, `resource`, `result`
- Uses `sync.Mutex` for serialized writes

### 6. Handlers (`internal/api/`)
- `HealthHandler` — returns `{"status":"ok"}` (no auth)
- `WhoamiHandler` — returns `{"role":"...","subject":"..."}` from JWT claims
- `ProjectsHandler` — method-dispatching handler that delegates to the permission-guarded handler for the method

## Routing

```
/health  ──▶ HealthHandler (unauthenticated)

/whoami  ──▶ AuthMiddleware ──▶ WhoamiHandler
/projects ─▶ AuthMiddleware ──▶ ProjectsHandler
                │
                ├── GET    ──▶ RequirePermission("read")    ──▶ ProjectsHandler
                ├── POST   ──▶ RequirePermission("write")   ──▶ ProjectsHandler
                └── DELETE ──▶ RequirePermission("delete")  ──▶ ProjectsHandler
```

## Data Flow

```
Request
  │
  ▼
AuthMiddleware.ExtractToken(request) ──▶ Bearer token string
  │
  ▼
auth.ValidateToken(token) ──▶ CustomClaims{role: "admin", ...}  OR  error ──▶ 401
  │
  ▼
context.WithValue(ctx, ClaimsKey, claims)
  │
  ▼
RequirePermission(action, resource) ──▶ Logger.Log(event)
  │                                    │
  ▼                                    ▼
rbac.Authorize(role, action)       audit.log
  │
  ├── true  ──▶ handler.ServeHTTP ──▶ 200
  └── false ──▶ 403 Forbidden
```

## Security Considerations

- JWT secret is a placeholder (`your-highly-secure-secret-key-change-in-production`) — must be replaced before any non-local deployment
- No token refresh mechanism — clients must re-authenticate
- Audit log is append-only and local to the process — production deployment should ship to a centralized log aggregator
- Context keys use an unexported struct type (`contextKey{}`) to prevent collisions
- No rate limiting or brute-force protection on the auth endpoint
