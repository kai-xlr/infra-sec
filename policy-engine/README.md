# policy-engine

Policy evaluation engine built in Rust.

## Architecture

```
src/
  policy.rs     — policy data structures
  evaluator.rs  — policy evaluation engine
  parser.rs     — policy file parsing (WIP)
  cache.rs      — decision caching (WIP)
  lib.rs        — public API
benches/        — criterion benchmarks
tests/          — integration tests
```

## Usage

```rust
use policy_engine::{evaluator::Engine, policy::Policy};

let engine = Engine::new(vec![
    Policy::new("admin", "write"),
    Policy::new("admin", "delete"),
    Policy::new("developer", "write"),
    Policy::new("viewer", "read"),
]);

assert!(engine.evaluate("viewer", "read"));
assert!(!engine.evaluate("viewer", "write"));
```

## Benchmarks

```text
single evaluation    — ns/op, allocations
1000 evaluations     — ns/op, allocations
10000 evaluations    — ns/op, allocations
```

Run with `cargo bench`.
