// Concept: Multi-worker Fan-out
//
// This example demonstrates distributing work from a single producer
// across multiple worker threads using channels, with verification
// that all items are processed.
//
// Key concepts:
//   Fan-out — One producer distributes work to N consumers.
//             Each worker pulls from the same channel (load-balancing).
//             Workers compete for items; no item is processed twice.
//   Fan-in  — Multiple producers send results to a single collector.
//             The inverse of fan-out. Often combined (fan-out → process → fan-in).
//   Work stealing vs distribution:
//     Distribution — Central queue, workers pull when idle. Simple, fair.
//                    Workers are idle only when queue is empty.
//     Work stealing — Each worker has a local queue. Idle workers steal
//                     from busy workers. Better locality, lower contention.
//                     Used by rayon, tokio.
//   Backpressure — Bounded channels block the producer when the buffer is full.
//                   Prevents unbounded memory growth when producers are faster
//                   than consumers. The producer blocks until a worker consumes.
//   mpsc fan-out — Clone the Sender N times. Each worker receives via the
//                  shared Receiver. When a worker calls recv(), it gets the
//                  next available item. No item is duplicated.
//   Graceful shutdown — Send a poison pill (None) to each worker, or drop
//                       all senders so recv() returns Err. Workers then
//                       join and report their results.

use std::sync::mpsc;
use std::thread;

fn main() {
    const TOTAL_ITEMS: u64 = 1000;
    const NUM_WORKERS: usize = 4;

    // ----- Phase 1: Single worker (baseline) -----
    println!("--- Single worker baseline ---");
    let (tx, rx) = mpsc::channel::<Option<u64>>();

    let producer = thread::spawn(move || {
        for i in 1..=TOTAL_ITEMS {
            tx.send(Some(i)).unwrap();
        }
        // Signal the worker to stop.
        drop(tx);
    });

    let worker = thread::spawn(move || {
        let mut count = 0u64;
        let mut sum = 0u64;
        for msg in rx {
            match msg {
                Some(val) => {
                    sum += val;
                    count += 1;
                }
                None => break,
            }
        }
        (count, sum)
    });

    producer.join().unwrap();
    let (count, sum) = worker.join().unwrap();
    println!("  Worker processed {count} items, sum = {sum}");

    // ----- Phase 2: Fan-out to N workers -----
    // All workers share the same Receiver. When a worker calls recv(),
    // it gets the next item — no work is duplicated. This is load-balancing
    // by competition: faster workers naturally process more items.
    println!("\n--- Fan-out: {NUM_WORKERS} workers ---");
    let (tx, rx) = mpsc::channel::<Option<u64>>();
    let rx = std::sync::Arc::new(std::sync::Mutex::new(rx));

    let mut handles = vec![];

    for id in 0..NUM_WORKERS {
        let rx = std::sync::Arc::clone(&rx);
        let handle = thread::spawn(move || {
            let mut count = 0u64;
            let mut sum = 0u64;
            loop {
                // Lock the mutex to receive from the shared channel.
                // This serializes recv() calls — only one worker can
                // receive at a time, but the actual processing happens
                // outside the lock.
                let msg = {
                    let guard = rx.lock().unwrap();
                    guard.recv()
                };
                match msg {
                    Ok(Some(val)) => {
                        sum += val;
                        count += 1;
                    }
                    Ok(None) | Err(_) => break,
                }
            }
            println!("  Worker {id} processed {count} items, sum = {sum}");
            (count, sum)
        });
        handles.push(handle);
    }

    // Send all work items, then drop tx so workers see channel close.
    for i in 1..=TOTAL_ITEMS {
        tx.send(Some(i)).unwrap();
    }
    drop(tx);

    let mut total_count = 0u64;
    let mut total_sum = 0u64;
    for handle in handles {
        let (count, sum) = handle.join().unwrap();
        total_count += count;
        total_sum += sum;
    }

    println!("  Total: {total_count} items processed, sum = {total_sum}");
    assert_eq!(total_count, TOTAL_ITEMS);
    assert_eq!(total_sum, TOTAL_ITEMS * (TOTAL_ITEMS + 1) / 2);
    println!("  Verification passed!");

    // ----- Phase 3: Backpressure with bounded channel -----
    // A bounded channel blocks the producer when the buffer is full.
    // This prevents the producer from flooding workers and consuming
    // unbounded memory. The producer will block until a worker pulls.
    println!("\n--- Bounded channel (capacity = 16) ---");
    let (tx, rx) = mpsc::sync_channel::<u64>(16);
    let rx = std::sync::Arc::new(std::sync::Mutex::new(rx));

    let mut handles = vec![];

    for id in 0..NUM_WORKERS {
        let rx = std::sync::Arc::clone(&rx);
        let handle = thread::spawn(move || {
            let mut count = 0u64;
            loop {
                let msg = {
                    let guard = rx.lock().unwrap();
                    guard.recv()
                };
                match msg {
                    Ok(val) => {
                        count += 1;
                        // Simulate work to show backpressure in action.
                        thread::yield_now();
                        let _ = val;
                    }
                    Err(_) => break,
                }
            }
            println!("  Worker {id} processed {count} items");
            count
        });
        handles.push(handle);
    }

    // Producer is blocked by sync_channel when buffer is full.
    // This naturally rate-limits the producer to worker speed.
    for i in 1..=TOTAL_ITEMS {
        tx.send(i).unwrap();
    }
    drop(tx);

    let mut total = 0u64;
    for handle in handles {
        total += handle.join().unwrap();
    }
    assert_eq!(total, TOTAL_ITEMS);
    println!("  Bounded verification passed! Total: {total}");

    // ----- Phase 4: Fan-in pattern -----
    // Multiple workers produce results that are collected by a single
    // aggregator. This is the inverse of fan-out.
    println!("\n--- Fan-in: workers → aggregator ---");
    let (result_tx, result_rx) = mpsc::channel::<(usize, u64)>();
    let mut worker_handles = vec![];

    // Partition 100 work items across NUM_WORKERS workers.
    // Each worker gets its own channel (Receiver can't be cloned).
    for id in 0..NUM_WORKERS {
        let result_tx = result_tx.clone();
        let start = (id as u64) * (100 / NUM_WORKERS as u64) + 1;
        let end = ((id as u64) + 1) * (100 / NUM_WORKERS as u64);
        let handle = thread::spawn(move || {
            for val in start..=end {
                // Simulate processing — square the value.
                result_tx.send((id, val * val)).unwrap();
            }
        });
        worker_handles.push(handle);
    }
    // Drop the original sender so the aggregator sees channel close.
    drop(result_tx);

    for h in worker_handles {
        h.join().unwrap();
    }

    // Aggregate results from all workers.
    let results: Vec<(usize, u64)> = result_rx.into_iter().collect();
    let fan_in_sum: u64 = results.iter().map(|(_, v)| v).sum();
    // Sum of squares: 1² + 2² + ... + 100² = 338350
    println!(
        "  Fan-in collected {} results, sum of squares = {fan_in_sum}",
        results.len()
    );
    assert_eq!(results.len(), 100);
    assert_eq!(fan_in_sum, 338350);
    println!("  Fan-in verification passed!");
}
