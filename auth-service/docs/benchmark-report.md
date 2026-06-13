# Benchmark Report — Week 1

**Date:** 2026-06-12
**Environment:** Linux x86_64, Go 1.25.2, Rust 1.93.0

## 1. Go Auth Service — HTTP Latency

50 sequential requests per endpoint on `localhost:8080`.

| Endpoint | Auth | Avg | Min | Max |
|----------|------|-----|-----|-----|
| `GET /health` | None | 0.6 ms | 0.5 ms | 1.1 ms |
| `GET /whoami` | JWT (admin) | 0.7 ms | 0.6 ms | 1.0 ms |
| `GET /projects` | JWT (admin) | 0.7 ms | 0.6 ms | 1.7 ms |
| `POST /projects` | JWT (viewer, **denied**) | 0.8 ms | 0.7 ms | 1.2 ms |

### Observations

- Auth middleware adds ~100 µs overhead over a bare health check
- Denied requests (403) are slightly slower than allowed ones — the audit log write happens before the 403 response
- All endpoints comfortably under 2 ms in sequential testing

## 2. Rust Policy Engine — Evaluation Throughput

Criterion benchmark, compiled with `--release`.

| Workload | Time | Throughput |
|----------|------|------------|
| Single evaluation | 29.9 ns | ~33M evals/s |
| 1000 evaluations | 30.2 µs | ~33M evals/s |
| 10000 evaluations | 274.7 µs | ~36M evals/s |

### Observations

- Linear scaling — `O(n)` expected for the current `iter().any()` implementation
- No meaningful overhead beyond the policy set size
- At 30 ns per evaluation, the engine is unlikely to be a bottleneck in any realistic HTTP workload

## 3. Cross-Project Comparison

| Metric | Go (auth+z) | Rust (eval only) |
|--------|-------------|-------------------|
| Latency per decision | ~700 µs (incl. HTTP, JWT parse, audit log) | ~30 ns |
| Decision throughput | ~1,400 req/s (single-thread) | ~33M evals/s |
| Bottleneck | JWT parsing + audit I/O | N/A |

The Rust engine is ~23,000× faster per evaluation, but in practice the Go HTTP overhead dominates. The Rust engine will only matter when decision counts per request grow (complex policy trees, ABAC, etc.).

## 4. Recommendations

- Current implementation is fine for Phase 1 scale
- If per-request evaluation counts exceed ~1,000, move the engine to Rust and call via FFI or sidecar
- Consider connection pooling or async I/O for the audit logger to reduce the denied-request latency penalty
