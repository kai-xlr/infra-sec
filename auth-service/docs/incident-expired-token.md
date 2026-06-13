# Incident Report: Expired Token Acceptance

**Report ID:** IR-2026-001
**Date:** 2026-06-12
**Severity:** Medium
**Component:** `auth-service/internal/auth/auth.go`

## Summary

A token with an expired `exp` claim was presented to the `/whoami` endpoint. The server correctly rejected it with a `401 Unauthorized` response. This report documents the test, confirms proper handling, and outlines the validation logic.

## Timeline

| Time | Event |
|------|-------|
| T+0s | Attacker generates a JWT with `exp` set to 1 hour in the past |
| T+1s | Request sent to `GET /whoami` with `Authorization: Bearer <expired token>` |
| T+1s | `auth.ValidateToken()` — `jwt.ParseWithClaims()` returns success (signature valid) |
| T+1s | Custom validation: `claims.ExpiresAt.Time.Before(time.Now())` → `true` |
| T+1s | Response: `401 Unauthorized` body. Audit event logged. |

## Root Cause

None — the token was intentionally expired. The validation logic correctly caught it.

## Detection

Manual test using an expired admin JWT:

```
$ curl -s -w "\nHTTP %{http_code}" \
    -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
    localhost:8080/whoami
Unauthorized
HTTP 401
```

A valid token on the same endpoint returned `200 OK` with claims, confirming the issue was expiration, not a general auth failure.

## Validation Logic (auth.go:39-45)

```go
if claims.ExpiresAt == nil {
    return nil, errors.New("missing expiration")
}

if claims.ExpiresAt.Time.Before(time.Now()) {
    return nil, errors.New("token expired")
}
```

The JWT library (`jwt.ParseWithClaims`) also checks `exp` internally when using `RegisteredClaims`, providing defence in depth. The explicit check adds a clearer error message and ensures the check is present even if library behaviour changes.

## Lessons Learned

- Expiration validation is implemented correctly.
- `exp` is a required claim — tokens without it are also rejected.
- No token refresh endpoint exists yet; clients must obtain a new token out of band.

## Action Items

| Item | Owner | Status |
|------|-------|--------|
| Document token lifetime expectations in API docs | — | Open |
| Consider adding a `POST /refresh` endpoint for transparent token rotation | — | Future |
