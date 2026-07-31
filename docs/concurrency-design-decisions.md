# Concurrency Design Decisions

**Scope:** Rust policy-engine + Go auth-service, Phase 1 concurrency work.

## 1. Policy Engine: Vec Linear Scan vs HashMap

### The design

`Engine` in `policy-engine/src/evaluator.rs` deliberately stores **both** representations:

```rust
pub struct Engine {
    policies: HashMap<(String, String), bool>, // O(1) path — evaluate()
    raw_policies: Vec<Policy>,                // O(n) path — evaluate_linear()
}
```

The HashMap collapses every policy into `(role, action) → allowed`, keyed by a tuple of two heap-allocated `String`s. The Vec keeps the original `Policy` structs (`{role, action, effect}`) untouched.

### Memory tradeoff

| Cost | HashMap path | Vec path |
|---|---|---|
| Extra allocations | One `String` per key component per lookup (`(role.to_string(), action.to_string())`); plus internal capacity, hash seed, and bucket overhead | Zero — iterates the pre-existing `Policy` structs in place |
| Key duplication | `role` + `action` strings duplicated between HashMap keys and Vec structs (2× string storage) | Single copy |
| Per-policy footprint | ~200+ bytes (2 heap Strings + hasher bookkeeping) | ~96 bytes (3 Strings inline in struct) |
| Cache locality | Random-access hashing, scattered cache lines | Sequential scan — prefetcher-friendly |

For the benchmark set (150 policies) the HashMap allocates ~30KB of key material plus hashing overhead vs zero additional allocation for linear.

### Speed tradeoff

Benchmarks from `benches/benchmark.rs` (Criterion, `--release`):

| Workload | HashMap (`evaluate`) | Linear (`evaluate_linear`) |
|---|---|---|
| 4 policies, single eval | ~30 ns | ~30 ns |
| 150 policies, single eval | ~30 ns (flat) | scales with `n` |
| 1000 evaluations (4 policies) | ~30 µs | ~30 µs |

At 4 policies the two are indistinguishable — a linear scan of 4 structs is cheaper than hashing two Strings. The HashMap only pays off past ~10–20 policies, and even then the marginal gain is tens of nanoseconds.

### Why keep both?

1. **Correctness reference.** `evaluate_linear` is the obviously-correct implementation against the raw policy list. The HashMap is a derived index that could silently diverge (e.g., a policy added to one path but not the other). Keeping both lets tests assert they always agree (see `tests/integration_test.rs` and the `evaluate`/`evaluate_linear` pairs in `evaluator.rs`).
2. **Shared ownership is free.** Both take `&self` — no locks, no mutation. The engine is `Send + Sync` and shareable across threads as long as the `RoleHierarchy` reference is also read-only. This is the "no locks" concurrency strategy: make the data immutable so no synchronization is needed.
3. **Hot-path readiness.** If Phase 2+ moves the engine into a hot decision loop (ABAC, complex policy trees), the HashMap path is already benchmarked and available.

**Decision:** Keep both. The memory cost of the index is ~200 bytes/policy and buys an O(1) path that costs nothing to maintain. The linear Vec is retained as the source of truth.

## 2. Decision Cache: Mutex vs RwLock vs Dashmap

### Current state

`PolicyCache` in `policy-engine/src/cache.rs` holds `HashMap<(String, String), CacheEntry>` with TTL-based lazy eviction (`evict_expired()` runs on every `get`). **Critically, it has no interior synchronization** — all methods take `&mut self`, so it is not `Sync` and cannot be shared across threads without external locking.

To share a cache across threads (or between handler goroutines in Go), three options exist:

### Option A: `Mutex<PolicyCache>`

```rust
let cache = Arc<Mutex<PolicyCache>>::new(PolicyCache::new(Duration::from_secs(60)));
```

- **Pros:** Simple, one lock type to reason about, no reader/writer distinction to get wrong. Exactly what `concurrent-cache.rs` demonstrates with `Arc<Mutex<HashMap<String, String>>>`.
- **Cons:** Readers block each other. Every `get()` serializes even when no writer is active. Under read-heavy policy evaluation (the common case — thousands of reads per write), throughput is needlessly capped.

### Option B: `RwLock<PolicyCache>`

```rust
let cache = Arc<RwLock<PolicyCache>>::new(...);
// readers:  cache.read().unwrap().get(role, action)
// writers:  cache.write().unwrap().set(role, action, decision)
```

- **Pros:** N readers proceed in parallel; only writers exclude. Matches the cache's actual access pattern (read-mostly, TTL eviction writes occasionally). This is the same logic that led the Go auth-service to use `sync.RWMutex` on `InMemoryStore` and `SQLiteStore`.
- **Cons:** Writer starvation under sustained reader load (readers keep the read lock and the writer waits indefinitely in std's default policy). Eviction writes need write access, so every TTL expiry cycle briefly blocks all readers.

### Option C: `dashmap` (sharded)

- **Pros:** Shards the map into N independent locks keyed by hash bucket; concurrent reads *and* writes to different keys proceed in parallel. Best raw throughput for high-contention caches.
- **Cons:** Third-party dependency (the repo currently uses **zero** non-std concurrency crates — no tokio, rayon, crossbeam, or dashmap). Higher memory footprint per shard. `get_mut` / value-reference semantics differ from std `HashMap`, adding API friction. Not justified at policy-engine scale.

### Recommendation

Use **Option B (`RwLock`)** when the cache becomes shared. The Go side already validated the read-heavy pattern with `sync.RWMutex` in `internal/store/store.go`. Defer dashmap until benchmarks show the RwLock read path as a bottleneck — the current `PolicyCache` API (`&mut self`) means the wrapping decision can be deferred without API churn.

**Anti-pattern to avoid** (already present in the codebase): `fan-out.rs` wraps an `mpsc::Receiver` in `Arc<Mutex<...>>` — a Mutex over a single-consumer type. That is necessary because `Receiver` is `!Sync`, but note the pattern: **lock to receive, drop the guard, then process outside the lock**. The critical section is one `recv()` call, not the work. Copy this discipline wherever locks wrap shared structures.

## 3. Thread Pool: Channel-based vs Rayon vs Manual

### Channel-based (what `thread-pool.rs` implements)

```rust
struct ThreadPool {
    workers: Vec<thread::JoinHandle<()>>,
    sender: Option<mpsc::Sender<Box<dyn FnOnce() + Send>>>,
}
```

- Fixed N workers, each looping on `recv()` from a shared `Arc<Mutex<Receiver>>`
- Jobs are `Box<dyn FnOnce() + Send>` — consumed exactly once, moved across threads
- Graceful shutdown via `Drop`: drop the sender (channel closes) → workers see `recv()` return `Err` → break → `join()`
- Simple, explicit, no dependencies

**Costs:** contention on the single `Mutex<Receiver>` for job dispatch; the executor does not rebalance work between workers (a slow worker stalls its queued jobs even if others are idle).

### Rayon

- **Work-stealing:** each worker owns a local deque; idle workers steal from busy ones. Better load balance and cache locality for CPU-bound parallel iterators (`par_iter`)
- Automatic for data-parallel loops; no manual worker management
- **Cons:** not a general executor — jobs must be unit-free closures over borrowed data (`Fn`/`Scope`), poor fit for heterogeneous, long-running, or blocking workloads; pulls in a dependency

### Manual OS threads vs runtime

The current design uses `std::thread` (1:1 OS threads). In an async context (tokio), blocking jobs would instead go through `tokio::task::spawn_blocking`. For this project — no async runtime, blocking `thread::sleep`, bounded job counts — 1:1 threads are correct.

### Decision

Manual channel-based pool (what exists). Rayon is the upgrade path only when CPU-bound policy evaluation becomes the dominant workload. The channel-based design also cleanly ports to the Go side, where the idiomatic equivalent is a goroutine pool consuming from a buffered channel.

## 4. Mutex vs Channels: Shared State vs Message Passing

### The framework

| Concern | Use `Mutex`/`RwLock` | Use channels |
|---|---|---|
| Data shape | Small, shared state: counters, caches, lookup tables | Work items, ownership transfer, event streams |
| Access pattern | Many threads touch the *same* cell | Producer/consumer, one-way flow |
| Lifecycle | Data lives as long as the process | Values move; channel close is a shutdown signal |
| Synchronization need | Mutate in place (read-modify-write) | Hand off ownership, consumer owns it after `recv()` |
| Backpressure | Manual (bounded structs, condvars) | Free with `sync_channel` (blocking send) |

Guiding rule: **if you need the result to be shared mutable state, use a lock; if you need to transfer ownership or distribute work, use a channel.**

### Codebase examples

- **Mutex (shared state):** `shared-counter.rs` — 8 threads increment one `u64`; the counter must be a shared cell. `concurrent-cache.rs` — one `HashMap` shared by 4 writers + 4 readers; a channel cannot share a map. Go: `sync.RWMutex` in `InMemoryStore`/`SQLiteStore`, `sync.Mutex` in `audit.Logger`.
- **Channels (ownership/work):** `mpsc-queue.rs` — `Command` values *move* through the channel; the producer can no longer touch a sent value. `fan-out.rs` — work distribution to N workers; the `Receiver`'s single-consumer nature is exactly the load-balancing semantics wanted. Go: `worker/cleanup.go` uses a `done chan struct{}` purely as a shutdown signal — the channel carries **zero data**, only completion. This is the cheapest form of signaling and the reason channels exist even when there's no "work".

### Anti-patterns

- **Mutex-held-while-awaiting:** holding a lock while calling `recv()` (or blocking I/O) serializes the whole system and risks deadlock. `fan-out.rs` and `thread-pool.rs` both scope the lock to the `recv()` call only. Keep this invariant.
- **Channel-where-a-shared-cell-fits:** using an mpsc to emulate shared state ("request-reply") fights the single-consumer constraint. If multiple threads need the same value, that's a lock or an atomic.
- **Mutex-where-a-channel-fits:** `retry-queue.rs` uses `Arc<Mutex<Vec<DeadLetterJob>>>` for the DLQ. A channel would also work, but the Mutex is right here because the DLQ is *shared* state (the `run` loop and `drain_dead_letters` both mutate it).

### Decision

Use the existing split: locks for shared mutable state, channels for ownership transfer, work distribution, and shutdown signaling. The Go side should mirror this — note the Go `Store` interface wraps mutations in `RWMutex`, while the background worker communicates solely via a `done` channel.

## 5. Go vs Rust Concurrency

### Goroutines vs OS threads

| Aspect | Go (goroutines) | Rust (std::thread) |
|---|---|---|
| Scheduling | M:N — many goroutines multiplexed onto OS threads | 1:1 — every `thread::spawn` is an OS thread |
| Stack | Starts ~2KB, grows/shrinks dynamically | 2MB default, fixed |
| Max scale | 100k+ goroutines per process is routine | Hundreds-to-thousands of threads before scheduling costs hurt |
| Model | `net/http` spawns a goroutine **per request** implicitly | No built-in per-request model; explicit pools |
| Costs | Cheap spawn, GC pressure from closures | Expensive spawn (~50µs), kernel involvement |

The Go auth-service leans on goroutine-per-request implicitly (`http.Server` does it). The only explicit concurrency is `CleanupWorker.Start()` — one goroutine, driven by a `time.Ticker` and a `done` channel, stopped by `close(workerDone)` on graceful shutdown.

The Rust side cannot rely on cheap implicit spawning; the explicit `ThreadPool` is the equivalent discipline — bound the thread count, distribute via channel, shut down cleanly in `Drop`.

### Channels: Go channel vs Rust mpsc

| Aspect | Go `chan T` | Rust `std::sync::mpsc` |
|---|---|---|
| Directionality | **Multi-producer, multi-consumer** (MPMC) | Multi-producer, **single-consumer** (SPSC+ — one `Receiver`) |
| Close semantics | `close(chan)` broadcast; receivers get zero-value forever | Dropping all `Sender`s closes; `recv()` returns `Err` |
| Select | `select` over N channels built-in | None in std — needs `crossbeam::select!` or tokio |
| Backpressure | Buffered channel blocks on send when full | `sync_channel` (bounded) blocks on send; unbounded never blocks |
| Ownership | Values are copied/aliased — data is GC-managed | Values are **moved**; compile-time ownership transfer |

The fan-out pattern exposes the difference: Go can share one `chan` across N consumer goroutines directly (MPMC). Rust's `mpsc` `Receiver` cannot be cloned, so `fan-out.rs` wraps it in `Arc<Mutex<Receiver>>` — the mutex is a workaround for single-consumer, serializing only the `recv()`.

Go's `select` over multiple channels has no std Rust equivalent; `cleanup.go`'s `select { case <-ticker.C: ...; case <-w.done: ... }` would require `crossbeam` or a runtime in Rust.

### Safety models

| Aspect | Go | Rust |
|---|---|---|
| Data-race detection | `-race` flag (runtime instrumentation) | Compile-time `Send`/`Sync`; races are compile errors |
| Reference sharing | Pointers shared freely; GC protects lifetimes | Borrow checker + `Arc` + lifetimes |
| Runtime failures | Panic crashes process unless recovered; `recover()` is opt-in | Panics unwind the thread; `JoinHandle::join()` returns the panic for inspection |

The `Rc vs Arc` block in `shared-counter.rs` is the canonical example: `Rc<Mutex<u64>>` across threads is a **compile error** in Rust (`Rc` is `!Send`), while the equivalent mistake in Go compiles and is caught only by `-race` or a crash.

### Decision

Keep goroutines + native channels on the Go side and explicit thread pools + mpsc on the Rust side. When a pattern needs to cross the boundary, translate the *structure* (pool → channel → workers → shutdown) rather than the primitives — MPMC Go channels and mpsc Rust receivers have different semantics and different fan-out workarounds.

## 6. Failure Scenarios

### Deadlock

**Mechanism:** Two threads acquire locks in different orders, or a thread blocks while holding a lock.

**In this codebase:**
- `fan-out.rs`/`thread-pool.rs`: lock `Mutex<Receiver>` then call `recv()`. If the guard were held *across* processing, a slow worker would hold the lock while others waited on `recv()` — deadlock-adjacent. The scoped-guard pattern (`let msg = { guard.recv() };`) prevents it.
- Go `SQLiteStore`: a `sync.RWMutex` wraps every operation. If a handler ever called two store methods while holding the store's lock (e.g., `CreateUser` then `GetUser` inside one lock), the second `Lock()` would self-deadlock. No method nests, but the interface makes no guarantee — worth documenting.
- Go `audit.Logger`: `sync.Mutex` held during JSON marshal + file write. If `Log` were ever called from a handler that already held the audit mutex, deadlock. Currently single-layer, but the file I/O inside the lock is the risk area (see `benchmark-report.md`: denied requests are slower because audit write happens before the 403).

**Mitigations:** acquire locks for the shortest span; never call out to unknown code under a lock; order lock acquisition consistently; keep I/O out of critical sections where possible.

### Starvation

**Mechanism:** A thread is perpetually denied the lock.

- Rust std `RwLock`: default policy is not fair — sustained readers can starve a writer. The TTL-eviction writes in `PolicyCache` would be the victim under heavy read load. Mitigation: use `RwLock` only when writes are rare and bounded, or accept eviction occurring on a separate low-frequency schedule.
- Go `sync.Mutex`: since Go 1.8, mutexes enter "starvation mode" — after ~1ms of waiting, the goroutine is queued FIFO ahead of new arrivals. Self-correcting; nothing to do.
- The Go rate limiter (`x/time/rate`, 5/s burst 10) is a **fairness device** for the login endpoint — a global token bucket that, when exhausted, effectively starves *all* clients until refill. That is intended here, but note it starves legitimate users too (see service-design-review, Challenge 2).

### Contention

**Mechanism:** Threads spend more time waiting for the lock than doing work.

- `concurrent-cache.rs`: a single coarse `Mutex<HashMap>` over 4 writers + 4 readers. At 80,000 ops this still completes, but throughput is lock-bound. Header notes fine-grained (sharded) locking as the upgrade path.
- `SQLiteStore`: **double-locking** — application `RWMutex` *plus* SQLite's internal serialization. Every write is serialized twice and reads block during writes despite SQLite supporting concurrent readers in WAL mode (not enabled). The app-level lock adds overhead without correctness benefit.
- `PolicyCache` HashMap lookups allocate two `String`s per `get` (key construction) — the allocation, not the lock, is the contention-adjacent cost. Using borrowed keys (`HashMap<&str,...>`) or a key type like `(String, String)` reused across calls would cut it.

**Mitigations:** measure before sharding; drop redundant locks (SQLite's internal one suffices); avoid per-call allocations on hot paths.

### Poisoning

**Mechanism (Rust):** A thread panics while holding a `Mutex`. The lock is marked poisoned; every subsequent `lock()` returns `Err(PoisonError)`.

- `shared-counter.rs` and `concurrent-cache.rs` both call `.lock().unwrap()`, which **propagates the poison** — after one panicked worker, the whole cache is unusable (every `unwrap` panics).
- `fan-out.rs` and `thread-pool.rs` use `.lock().unwrap()` on the receiver mutex; a worker panic while holding it would poison job distribution. `ThreadPool::drop` ignores join errors (`let _ = worker.join()`), so a panicked worker is silently replaced by a missing worker — the pool shrinks without notice.
- `retry-queue.rs` `drain_dead_letters` does `&mut *self.dead_letters.lock().unwrap()` — a poisoned DLQ would panic on the first drain.

**Poisoning is a feature, not just a bug:** it surfaces "state may be corrupt" at the point of use. Choose deliberately: `.unwrap()` (propagate — fail fast), `.into_inner()` (recover the value, ignore poison), or `map_err`/`expect` with context.

**Mechanism (Go):** No poisoning concept. An unrecovered panic kills the whole process; a `recover()`ed panic leaves the mutex **locked forever** — subsequent `Lock()` calls deadlock silently. Go's failure mode is the inverse: state is never marked corrupt, so corruption can deadlock rather than error. Mitigation: never `recover()` across a mutex-protected region without a plan, and prefer failing fast.

### Summary table

| Failure | Rust behavior | Go behavior |
|---|---|---|
| Deadlock | `recv()` under lock, nested locks | Nested `Lock()` on same mutex, lock across I/O |
| Starvation | `RwLock` writer starvation (unfair) | Mutex starvation mode self-corrects |
| Contention | Coarse `Mutex<HashMap>`, per-call allocations | Double-locking (RWMutex + SQLite) |
| Poisoning | `PoisonError` — explicit, recoverable choice | No concept — `recover()` + lock = silent deadlock |
