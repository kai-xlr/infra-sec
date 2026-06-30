# Incident: Privilege Escalation Investigation

## Scope

Investigate whether a non-admin user (viewer role) can escalate privileges via the admin API (`/admin/users`).

## Test Summary

| Endpoint | Method | Result |
|---|---|---|
| `/admin/users` | POST (create user) | 403 Forbidden |
| `/admin/users/{id}` | PUT (update user) | 403 Forbidden |
| `/admin/users/{id}` | DELETE (delete user) | 403 Forbidden |
| `/admin/users` | GET (list users) | 403 Forbidden |
| `/admin/users/{id}` | GET (get user) | 403 Forbidden |

## Mitigation

The existing middleware chain is effective:

```
AuthMiddleware → RequireRole("admin") → adminMux
```

- `AuthMiddleware` validates the JWT and injects `*models.CustomClaims` into the request context.
- `RequireRole("admin")` reads claims from context; if the role is not `"admin"`, it returns **403 Forbidden**.

## Conclusion

**No privilege escalation vulnerability found.** A user with the `viewer` role cannot access any admin endpoint. All endpoints correctly return 403.
