// Concept: Arc<Mutex<HashMap>> Concurrent Cache
//
// This example demonstrates a thread-safe key-value cache using
// Arc<Mutex<HashMap<String, String>>> with concurrent readers and writers.
//
// Key concepts:
//   Mutex vs RwLock:
//     Mutex — exclusive access regardless of read vs write. Simple, predictable.
//             Best when writes are frequent (write-heavy workloads).
//     RwLock — allows many readers OR one writer at a time.
//              Best when reads dominate (read-heavy workloads).
//              Readers don't block each other, reducing contention.
//   Contention patterns:
//     Read-heavy: RwLock shines — multiple readers proceed in parallel.
//     Write-heavy: RwLock degrades — writers starve under reader load.
//                  Mutex is simpler and may perform similarly.
//   Lock granularity:
//     Coarse (one big lock): Simple, safe, but high contention.
//     Fine (e.g. sharding): Lower contention, higher complexity.
//     Here we use a single coarse lock for clarity.
//
// Poisoning: If a thread panics while holding the lock, the Mutex is poisoned.
//            Subsequent lock() calls return Err(PoisonError).

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Instant;

struct Cache {
    inner: Mutex<HashMap<String, String>>,
}

impl Cache {
    fn new() -> Self {
        Cache {
            inner: Mutex::new(HashMap::new()),
        }
    }

    fn get(&self, key: &str) -> Option<String> {
        let map = self.inner.lock().unwrap();
        map.get(key).cloned()
    }

    fn set(&self, key: String, value: String) {
        let mut map = self.inner.lock().unwrap();
        map.insert(key, value);
    }

    fn remove(&self, key: &str) -> Option<String> {
        let mut map = self.inner.lock().unwrap();
        map.remove(key)
    }

    fn snapshot(&self) -> HashMap<String, String> {
        self.inner.lock().unwrap().clone()
    }
}

fn main() {
    let cache = Arc::new(Cache::new());

    // Pre-populate with some keys for readers to find
    {
        let cache = cache.clone();
        let keys = ["a", "b", "c", "d", "e"];
        for (i, k) in keys.iter().enumerate() {
            cache.set(k.to_string(), format!("initial_{i}"));
        }
    }

    let mut handles = vec![];
    let start = Instant::now();
    const OPS_PER_THREAD: u32 = 10_000;

    // ----- Writer threads -----
    // Each writer continuously sets and removes keys. Writers introduce
    // churn — keys appear, disappear, and change values over time.
    for id in 0..4 {
        let c = Arc::clone(&cache);
        let handle = thread::spawn(move || {
            for i in 0..OPS_PER_THREAD {
                let key = format!("writer_{id}_{}", i % 20);
                if i % 5 == 0 {
                    c.remove(&key);
                } else {
                    c.set(key, format!("val_{id}_{i}"));
                }
            }
        });
        handles.push(handle);
    }

    // ----- Reader threads -----
    // Each reader continuously reads known keys (a-e) and dynamic keys
    // that writers may or may not have inserted. Reads should never
    // panic or observe inconsistent state.
    for _id in 0..4 {
        let c = Arc::clone(&cache);
        let handle = thread::spawn(move || {
            for i in 0..OPS_PER_THREAD {
                // Read a pre-populated key (always present)
                let _ = c.get("a");
                // Read a key that writers might have set
                let _ = c.get(&format!("writer_{}_{}", i % 4, i % 20));
                // Read a non-existent key
                let _ = c.get("nonexistent");
            }
        });
        handles.push(handle);
    }

    // Wait for all threads to complete
    for handle in handles {
        handle.join().unwrap();
    }

    let elapsed = start.elapsed();
    let total_ops = 8 * OPS_PER_THREAD; // 4 readers + 4 writers
    let throughput = total_ops as f64 / elapsed.as_secs_f64();

    // Final consistency check: take a snapshot and verify no corruption
    let snapshot = cache.snapshot();
    let mut ok = true;
    for (k, v) in &snapshot {
        // Every value should have the form "val_X_Y" for writer entries
        if !k.starts_with("writer_") {
            continue;
        }
        if !v.starts_with("val_") {
            eprintln!("CORRUPTION: key={k} has invalid value={v}");
            ok = false;
        }
    }
    if ok {
        println!("Data integrity: OK");
    }
    println!(
        "{} ops in {:.2}s — {:.0} ops/s",
        total_ops,
        elapsed.as_secs_f64(),
        throughput
    );
    println!("Final cache size: {} entries", snapshot.len());
}
