# policy-engine

Policy evaluation engine built in Rust.

## Architecture

```
src/
  policy.rs     — policy data structures
  parser.rs     — policy file parsing
  evaluator.rs  — policy evaluation logic
  cache.rs      — decision caching
  lib.rs        — public API
benches/        — benchmarks
tests/          — integration tests
```
