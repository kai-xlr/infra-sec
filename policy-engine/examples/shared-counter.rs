// Concept: Arc<Mutex> Shared Counter
//
// This example demonstrates interior mutability and shared ownership
// across threads using Arc<Mutex<u64>>.
//
// Key concepts:
//   Arc — Atomic Reference Counted pointer. Thread-safe shared ownership.
//         Use Arc when multiple threads need to share read/write access to
//         the same heap-allocated data. Clone increments the reference count.
//   Mutex — Mutual exclusion primitive. Guards data behind a lock.
//           Only one thread can hold the lock at a time.
//           lock() returns a MutexGuard which derefs to the inner data.
//   vs Rc — Rc is not thread-safe (no Send/Sync). Rc's reference count
//           uses non-atomic increments, causing data races across threads.
//   Poisoning — If a thread panics while holding the Mutex lock, the
//               Mutex becomes "poisoned". Subsequent lock() calls return
//               Err(PoisonError). Use .unwrap() to propagate or .into_inner()
//               to recover.

use std::sync::{Arc, Mutex};
use std::thread;

fn main() {
    // Arc provides thread-safe shared ownership of the Mutex.
    // Without Arc, each thread would own its own copy (no sharing).
    let counter = Arc::new(Mutex::new(0u64));

    let mut handles = vec![];

    // ----- Rc Version (DOES NOT COMPILE) -----
    // If we used Rc<Mutex<u64>> instead of Arc<Mutex<u64>>:
    //
    //   use std::rc::Rc;
    //   let counter = Rc::new(Mutex::new(0u64));
    //
    // thread::spawn(move || {
    //     *counter.lock().unwrap() += 1;
    // });
    //
    // Error: `Rc<Mutex<u64>>` cannot be sent between threads safely.
    //        the trait `Send` is not implemented for `Rc<Mutex<u64>>`.
    //
    // Rc uses non-atomic reference counting. When clone() increments the
    // count, another thread might read a stale value, causing undefined
    // behavior. Arc uses atomic increments (SeqCst ordering) so all threads
    // see a consistent reference count.
    // -----------------------------------------

    for _ in 0..8 {
        // Arc::clone increments the reference count (cheap, atomic).
        // Each thread gets its own Arc handle pointing to the same Mutex.
        let c = Arc::clone(&counter);

        let handle = thread::spawn(move || {
            for _ in 0..1000 {
                // lock() blocks until the Mutex is acquired.
                // The returned MutexGuard derefs to &mut u64 via DerefMut.
                // When the guard goes out of scope, the lock is released.
                let mut num = c.lock().unwrap();
                *num += 1;

                // MutexGuard dropped here — lock released, even if no explicit
                // scope block. If the thread panics here, the Mutex is poisoned.
            }
        });

        handles.push(handle);
    }

    for handle in handles {
        // join() blocks until the thread finishes.
        // If the thread panicked, join() returns Err. We unwrap to propagate.
        handle.join().unwrap();
    }

    let final_count = *counter.lock().unwrap();
    println!("Count: {final_count}");
}
