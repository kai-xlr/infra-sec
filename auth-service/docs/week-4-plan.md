# 🗓️ Week 4 — Integration Testing, API Enhancement & Policy Engine Expansion

**Reference:** [ROADMAP.md](../../ROADMAP.md)

This week bridges our early-completed auth + policy work into the Adjusted Roadmap's Phase 1.
After Week 4, we pivot to Rust concurrency + Go background worker + leadership artifacts.

## 🎯 Objective
Add integration tests for all Go API endpoints, enhance the auth API with user-facing endpoints, and expand the Rust policy engine with file loading, caching, hierarchy, and CI.

## 📦 Tickets

### Tier 1: Integration Tests (Tickets 1–2)

| # | Issue | File(s) | Behavior |
|---|---|---|---|
| 1 | [#42](https://github.com/kai-xlr/infra-sec/issues/42) | `auth-service/tests/auth_test.go` | `go test` against a test server: login valid/invalid, health, whoami, 401 without token |
| 2 | [#43](https://github.com/kai-xlr/infra-sec/issues/43) | `auth-service/tests/admin_test.go` | `go test` against a test server: full CRUD lifecycle with admin token, 403 with viewer token |

### Tier 2: API Enhancement (Tickets 3–6)

| # | Issue | File(s) | Behavior |
|---|---|---|---|
| 3 | [#44](https://github.com/kai-xlr/infra-sec/issues/44) | `internal/handler/auth.go` | Return current user's username and role from JWT claims — no DB lookup needed |
| 4 | [#45](https://github.com/kai-xlr/infra-sec/issues/45) | `internal/handler/auth.go` | `PUT /auth/password` with current+new password, bcrypt verify then re-hash |
| 5 | [#46](https://github.com/kai-xlr/infra-sec/issues/46) | `internal/handler/admin.go` | Optional `?role=viewer` query param filters results server-side |
| 6 | [#47](https://github.com/kai-xlr/infra-sec/issues/47) | `internal/model/`, `internal/store/` | Add field, default to `created_at`, auto-update on `UpdateUser` |

### Tier 3: Policy Engine Expansion (Tickets 7–10)

| # | Issue | File(s) | Behavior |
|---|---|---|---|
| 7 | [#48](https://github.com/kai-xlr/infra-sec/issues/48) | `policy-engine/src/parser.rs` | `parse_policy_file(path)` returns `Vec<Policy>`; validate required fields |
| 8 | [#49](https://github.com/kai-xlr/infra-sec/issues/49) | `policy-engine/src/cache.rs` | Cache policy decisions with configurable TTL; hit returns cached bool, miss evaluates + stores |
| 9 | [#50](https://github.com/kai-xlr/infra-sec/issues/50) | `policy-engine/src/evaluator.rs` | Replace `Vec` linear scan with `HashMap<(role, action), bool>` for O(1) evaluation |
| 10 | [#51](https://github.com/kai-xlr/infra-sec/issues/51) | `policy-engine/src/policy.rs` | `admin` inherits `developer` + `viewer` permissions; configurable hierarchy map |

### Tier 4: CI & Polish (Tickets 11–12)

| # | Issue | File(s) | Behavior |
|---|---|---|---|
| 11 | [#52](https://github.com/kai-xlr/infra-sec/issues/52) | `.github/workflows/rust.yml` | `cargo test` + `cargo bench` on push/PR; report test results |
| 12 | [#53](https://github.com/kai-xlr/infra-sec/issues/53) | `policy-engine/Cargo.toml`, `policy-engine/src/policy.rs` | `Policy` derives `Serialize`/`Deserialize`; load rules from JSON string |

---

## 🚀 Shipping Criteria

By end of week:
- [ ] Integration tests pass for all API endpoints
- [ ] `GET /me` returns user profile from JWT
- [ ] Password change works with old/new password
- [ ] ListUsers supports `?role=` filtering
- [ ] Users track `updated_at`
- [ ] Policy engine loads rules from JSON files
- [ ] Decision cache with configurable TTL
- [ ] HashMap-based evaluation (O(1))
- [ ] RBAC hierarchy (admin inherits all)
- [ ] Rust CI runs tests + benchmarks on push
- [ ] Serde serialization for Policy struct
