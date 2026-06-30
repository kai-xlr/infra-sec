# Phase 1 Remaining Plan — Post Week 4

**Reference:** [ROADMAP.md](../ROADMAP.md)
**Timeline:** July – August 2026 (~4 weeks)
**Pace:** 2–3 tickets/week, building capabilities not systems

---

## Mapping to Roadmap Capabilities

| ROADMAP.md Capability | Issue | Week |
|----------------------|-------|------|
| Shared counter | [#59](https://github.com/kai-xlr/infra-sec/issues/59) — Arc<Mutex> counter | 5 |
| Mutex protected state | [#60](https://github.com/kai-xlr/infra-sec/issues/60) — Concurrent cache | 5 |
| Message passing queue | [#61](https://github.com/kai-xlr/infra-sec/issues/61) — mpsc producer-consumer | 6 |
| Worker thread | [#62](https://github.com/kai-xlr/infra-sec/issues/62) — Single worker (first in fan-out) | 6 |
| Multi-worker queue | [#62](https://github.com/kai-xlr/infra-sec/issues/62) — Fan-out distribution | 6 |
| Graceful shutdown | [#54](https://github.com/kai-xlr/infra-sec/issues/54) — Thread pool Drop/shutdown | 7 |
| Retry queue | [#55](https://github.com/kai-xlr/infra-sec/issues/55) — Backoff + dead letter | 7 |
| Thread pool | [#54](https://github.com/kai-xlr/infra-sec/issues/54) — Generic job pool | 7 |
| Background worker (Go) | [#56](https://github.com/kai-xlr/infra-sec/issues/56) — Cleanup goroutine | 8 |
| Service Design Review | [#57](https://github.com/kai-xlr/infra-sec/issues/57) — Leadership artifact | 8 |
| Concurrency Design Decisions | [#58](https://github.com/kai-xlr/infra-sec/issues/58) — Leadership artifact | 8 |

## Overview

After Week 4 (integration tests, API enhancement, policy engine expansion), the remaining Phase 1 work focuses on:

1. **Rust concurrency fundamentals** — Shared state, channels, thread pools, retry patterns
2. **Go background worker** — Periodic tasks in the auth service
3. **Leadership artifacts** — Service Design Review + Concurrency Design Decisions
4. **Concept studies** — Written notes on ownership, borrowing, channels, mutex, contexts

---

## Week 5 — Rust Shared State

### Concept: Ownership & Borrowing
Study Rust's ownership model, borrowing rules, and how they prevent data races at compile time.

### Ticket A [#59]: `Arc<Mutex>` Shared Counter

**Goal:** Build a thread-safe counter using `Arc<Mutex<u64>>` that demonstrates interior mutability and shared ownership.

**Behavior:**
- Create `examples/shared-counter.rs` (or a `concurrency/` crate)
- Spawn 8 threads, each increments a shared counter 1000 times
- Join all threads, verify final count == 8000
- Include a version without `Arc` that fails to compile (commented out with explanation)
- Run with `cargo run --example shared-counter`

**Concept notes to produce:**
- What is `Arc`? Why not `Rc`?
- What is `Mutex`? What is `MutexGuard`?
- How does ownership prevent data races?
- What happens if a thread panics while holding the lock? (poisoning)

**Done when:** `cargo run --example shared-counter` prints `Count: 8000`

### Ticket B [#60]: `Mutex<HashMap>` Concurrent Cache

**Goal:** Build a thread-safe key-value cache using `Arc<Mutex<HashMap<String, String>>>` that handles concurrent reads and writes.

**Behavior:**
- Create `examples/concurrent-cache.rs`
- Implement `Cache` struct: `get(key) -> Option<String>`, `set(key, value)`, `remove(key)`
- Spawn reader threads (read loop) and writer threads (write loop) simultaneously
- Verify no panics, data consistency under concurrent access
- Measure throughput with `std::time::Instant`

**Concept notes to produce:**
- Why `Mutex<HashMap>` vs `RwLock<HashMap>`?
- Contention patterns under read-heavy vs write-heavy workloads
- Lock granularity tradeoffs

**Done when:** Concurrent read/write test runs without data loss or panics

---

## Week 6 — Rust Channels

### Concept: Channels vs Mutex
Study Go-inspired channel patterns in Rust (`std::sync::mpsc`) and when to prefer channels over shared state.

### Ticket C [#61]: `mpsc` Producer-Consumer Queue

**Goal:** Build a producer-consumer pattern using `std::sync::mpsc` channel demonstrating ownership transfer between threads.

**Behavior:**
- Create `examples/mpsc-queue.rs`
- Producer thread sends N messages (1..=1000)
- Consumer thread receives and sums them
- Verify sum == 500500
- Show typed messages with a custom enum (`Command::{Increment, Reset, Stop}`)
- Use `Stop` variant for graceful termination

**Concept notes to produce:**
- How does `mpsc` transfer ownership?
- What happens when the sender is dropped?
- Why `Send` and `Sync` matter
- Multiple consumers via `Mutex<Receiver>` or `mpsc::Sender` clones

**Done when:** `cargo run --example mpsc-queue` prints `Sum: 500500`

### Ticket D [#62]: Multi-worker Fan-out

**Goal:** Build a single-producer, multi-worker pattern where one source distributes work across N worker threads.

**Behavior:**
- Create `examples/fan-out.rs`
- One producer generates work items (strings to process)
- 4 worker threads receive via shared channel
- Workers report how many items they processed
- Add load-balancing: workers pull from shared queue, not assigned
- Verify total processed == total sent

**Concept notes to produce:**
- Fan-out vs fan-in patterns
- Work stealing vs work distribution
- Backpressure and bounded channels (using `crossbeam` or `sync_channel`)

**Done when:** All N workers collectively process all items, counts sum to total

---

## Week 7 — Rust Thread Pool & Retry

### Concept: Async vs Threads
Study the tradeoffs between OS threads and async tasks, and when each is appropriate for security infrastructure.

### Ticket E [#54]: Thread Pool with Graceful Shutdown

**Goal:** Build a generic thread pool that accepts `FnOnce` jobs and shuts down gracefully via channel-based signaling.

**Behavior:**
- Create `examples/thread-pool.rs`
- `ThreadPool::new(size: usize)` spawns N worker threads
- `pool.execute(job: impl FnOnce() + Send + 'static)` sends job to available worker
- On `Drop`, signal all workers to stop, join all threads
- Workers block waiting for jobs, not busy-loop
- Test: submit 20 jobs to 4 workers, verify all complete

**Concept notes to produce:**
- Design decisions: why `crossbeam::channel` vs `std::sync::mpsc`
- Graceful shutdown patterns
- Panic handling in worker threads
- Comparison with `rayon` and `tokio`

**Done when:** 20 jobs complete successfully with 4 workers; dropping pool waits for completion

### Ticket F [#55]: Retry Queue with Exponential Backoff

**Goal:** Build a worker queue that retries failed jobs with exponential backoff, capped retries.

**Behavior:**
- Create `examples/retry-queue.rs`
- `RetryQueue::new(max_retries: u32, base_delay: Duration)` 
- Submit jobs that may fail (configurable failure rate via RNG)
- Worker retries with `base_delay * 2^attempt` backoff
- After `max_retries` failures, move job to dead letter queue
- Report: succeeded, failed (retried), dead letter
- Test: submit 10 jobs with 50% failure rate, verify retries happen

**Concept notes to produce:**
- Backoff strategies: exponential, jitter, constant
- When retry makes sense in auth/policy systems
- Dead letter queues and observability

**Done when:** Retry queue handles failures with backoff; dead letter count is accurate

---

## Week 8 — Go Worker & Leadership Artifacts

### Concept: Go Contexts & Goroutines
Study Go's context package for cancellation and deadlines across goroutine boundaries.

### Ticket G [#56]: Go Background Worker

**Goal:** Add a background worker goroutine to the auth service that performs periodic token/session cleanup.

**Behavior:**
- Add `internal/worker/cleanup.go` with a `CleanupWorker` struct
- `NewCleanupWorker(store Store, interval time.Duration, done chan struct{})`
- Runs in a goroutine: every `interval`, scans for users past TTL (e.g., accounts not updated in 30 days) and logs them
- Respects context cancellation — stop when `done` is closed
- Wire in `main.go` using `context.WithCancel`
- Log cleanup activity with structured logging
- Configurable interval via `CLEANUP_INTERVAL` env var (default 24h)

**Learn:**
- Goroutine lifecycle management
- Context cancellation propagation
- Graceful shutdown of background workers
- Periodic timer patterns (`time.Ticker` vs `time.AfterFunc`)

**Done when:** Server starts, worker logs cleanup activity, server shutdown stops worker, no goroutine leaks

### Leadership Artifact H [#57]: Service Design Review

**Goal:** Write a service design review document covering the auth service architecture.

**Content:**
- Request lifecycle: incoming HTTP request → middleware chain → handler → store → response
- Middleware dependency graph (order matters)
- Error handling strategy: `jsonError` helper, error types, response codes
- Storage layer: Store interface, SQLite vs InMemory, migration strategy
- Security architecture: JWT validation, bcrypt cost, rate limiting, role checks
- What happens on restart: SQLite persistence, seed user, in-memory rate limiter resets
- Failure modes: SQLite corruption, bcrypt slowdown, rate limit exhaustion
- Tradeoffs: why SQLite vs Postgres, why pure Go SQLite vs CGo, why x/time/rate vs sliding window

**Format:** Markdown document in `auth-service/docs/service-design-review.md`

**Done when:** Document covers all sections; AI review challenges at least 3 assumptions

### Leadership Artifact I [#58]: Concurrency Design Decisions

**Goal:** Write a document analyzing concurrency choices across both services.

**Content:**
- Policy engine: why Vec<Policy> linear scan vs HashMap — memory vs speed tradeoff
- Decision cache design: Mutex<HashMap> vs RwLock<HashMap> vs dashmap
- Thread pool design: channel-based vs rayon vs manual
- Mutex vs Channels framework: when to use which (shared state vs message passing)
- Go vs Rust concurrency models: goroutines vs threads, channels vs mpsc
- Failure scenarios: deadlock, starvation, contention, poisoning

**Format:** Markdown document in `docs/concurrency-design-decisions.md`

**Done when:** Document covers all sections with concrete examples from our codebase
