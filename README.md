# infra-sec

Go & Rust security infrastructure: auth, policy, governance, and AI agent authorization.

## Projects

| Service | Language | Description |
|---|---|---|
| `auth-service/` | Go | JWT auth, RBAC, audit logging, REST API, SQLite persistence, integration tests |
| `policy-engine/` | Rust | Policy parser, evaluation engine, decision cache, Criterion benchmarks |

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
| 1 | Jun–Sep 2026 | Systems Foundations (Go + Rust) |
| 2 | Oct–Dec 2026 | Identity & Authorization Foundations |
| 3 | Jan–Mar 2027 | Policy Systems |
| 4 | Apr–Jun 2027 | Reliability & Distributed Thinking |
| 5 | Jul–Aug 2027 | Observability & Operations |
| 6 | Sep–Dec 2027 | AI Security Infrastructure |

Full roadmap: [ROADMAP.md](./ROADMAP.md)
