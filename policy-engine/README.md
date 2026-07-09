# policy-engine

Policy evaluation engine built in Rust.

## Architecture

```
src/
  policy.rs     — Policy struct, RoleHierarchy with default inheritance
  evaluator.rs  — HashMap O(1) + Vec linear evaluation engines
  parser.rs     — JSON policy file loading and validation
  cache.rs      — TTL-based decision caching
  lib.rs        — public API
examples/       — usage examples (shared-counter, concurrent-cache)
benches/        — criterion benchmarks (HashMap vs linear comparison)
tests/          — integration tests
```

## Usage

```rust
use policy_engine::{evaluator::Engine, policy::{Policy, RoleHierarchy}};

let engine = Engine::new(vec![
    Policy::new("admin", "write", "allow"),
    Policy::new("developer", "write", "allow"),
    Policy::new("viewer", "read", "allow"),
]);

let hierarchy = RoleHierarchy::default();
// admin inherits viewer read via hierarchy
assert_eq!(engine.evaluate("admin", "read", &hierarchy), Some(true));
// viewer cannot write
assert_eq!(engine.evaluate("viewer", "write", &hierarchy), None);
```

## Quickstart

```bash
# from repo root
make build-rust     # or: cargo build --manifest-path policy-engine/Cargo.toml
make test-rust      # or: cargo test --manifest-path policy-engine/Cargo.toml
make bench          # or: cargo bench --manifest-path policy-engine/Cargo.toml

# or just:
cd policy-engine && cargo test
```

## Benchmarks

Both `evaluate` (HashMap O(1)) and `evaluate_linear` (Vec O(n)) are benchmarked for comparison:

```text
HashMap single evaluation (4 policies)     ~110ns
linear single evaluation (4 policies)       ~26ns
HashMap single evaluation (150 policies)   ~146ns
linear single evaluation (150 policies)    ~639ns
```

HashMap overhead dominates on tiny sets, but at 100+ policies the O(1) lookup is ~4x faster.

Run with `make bench` or `cargo bench`.

## Examples

```bash
cargo run --example shared-counter    # Arc<Mutex<u64>> shared counter
cargo run --example concurrent-cache  # Arc<Mutex<HashMap>> concurrent KV cache
```
