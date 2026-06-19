# infra-sec

Go & Rust security infrastructure: auth, policy, governance, and AI agent authorization.

## Projects

| Service | Language | Description |
|---|---|---|
| `auth-service/` | Go | JWT auth, RBAC, audit logging, REST API, SQLite persistence |
| `policy-engine/` | Rust | Policy parsing, evaluation, decision caching |

## Quickstart

```bash
make build          # build all services
make test           # run all tests (Go + Rust)
make run            # start auth server on :8080
make e2e            # full integration test suite
make token ROLE=admin  # generate a test JWT
```

Requires Go ≥1.22 and Rust ≥1.75.

## Phases

| Phase | Timeline | Focus |
|---|---|---|
| 1 | Jun–Aug 2026 | Security Systems Foundations |
| 2 | Sep–Oct 2026 | Distributed Authorization Systems |
| 3 | Nov 2026 | Observability & Auditability |
| 4 | Dec 2026–Jan 2027 | Identity & Trust Infrastructure |
| 5 | Feb–Apr 2027 | Performance Engineering |
| 6 | Mar–May 2027 | AI Security Infrastructure |
