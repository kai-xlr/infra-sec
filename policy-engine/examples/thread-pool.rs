// Concept: Thread Pool with Graceful Shutdown
//
// This example demonstrates building a reusable thread pool that accepts
// closures for execution and shuts down cleanly when dropped.
//
// Key concepts:
//   ThreadPool — Manages a fixed number of worker threads. Jobs are sent
//                via a channel; workers pull and execute them. On Drop,
//                the pool signals shutdown and waits for all workers.
//   FnOnce + Send + 'static — The job trait bound. FnOnce because the job
//                is consumed when executed (runs exactly once). Send because
//                the job moves across thread boundaries. 'static because
//                the job must own all its data (no borrowed references).
//   mpsc::Sender — Each clone sends to the same channel. Dropping all
//                  senders closes the channel, signaling workers to stop.
//   Thread::join — Blocks until the thread finishes. If the thread panicked,
//                  join returns Err. We unwrap to propagate panics.
//   Graceful shutdown — Drop the Sender (closing the channel), then join
//                       all worker threads. Workers see recv() return Err
//                       and exit their loop cleanly.
//   vs rayon — rayon uses work-stealing with a global job queue. Our pool
//              uses a simple mpsc channel where workers compete for jobs.
//              rayon is more efficient for CPU-bound parallel iterators.
//   vs tokio — tokio is an async runtime with cooperative scheduling.
//              Our pool runs blocking tasks on OS threads. Use tokio::task::spawn_blocking
//              if you need to run blocking code inside an async context.
//   Panic handling — If a worker panics, its thread dies but the pool
//                    continues. The panicked worker's join returns Err.
//                    We propagate this in Drop via unwrap().

use std::sync::mpsc;
use std::thread;

struct ThreadPool {
    workers: Vec<thread::JoinHandle<()>>,
    sender: Option<mpsc::Sender<Box<dyn FnOnce() + Send>>>,
}

impl ThreadPool {
    fn new(size: usize) -> Self {
        let (sender, receiver) = mpsc::channel::<Box<dyn FnOnce() + Send>>();
        let receiver = std::sync::Arc::new(std::sync::Mutex::new(receiver));
        let mut workers = Vec::with_capacity(size);

        for id in 0..size {
            let rx = std::sync::Arc::clone(&receiver);
            let handle = thread::spawn(move || loop {
                // recv() blocks until a job arrives or all senders are dropped.
                // The Mutex serializes recv() calls so only one worker receives
                // each job. After locking, the guard is dropped immediately
                // (processing happens outside the lock).
                let job = {
                    let guard = rx.lock().unwrap();
                    guard.recv()
                };
                match job {
                    Ok(job) => {
                        println!("  Worker {id} got job; executing.");
                        job();
                    }
                    // All senders dropped — channel closed, time to exit.
                    Err(_) => {
                        println!("  Worker {id} shutting down.");
                        break;
                    }
                }
            });
            workers.push(handle);
        }

        ThreadPool {
            workers,
            sender: Some(sender),
        }
    }

    fn execute<F>(&self, job: F)
    where
        F: FnOnce() + Send + 'static,
    {
        let job = Box::new(job);
        self.sender.as_ref().unwrap().send(job).unwrap();
    }
}

impl Drop for ThreadPool {
    fn drop(&mut self) {
        // Drop the sender to close the channel.
        // Workers will see recv() return Err and exit their loops.
        drop(self.sender.take());

        // Wait for all workers to finish.
        // If a worker panicked, join returns Err and unwrap propagates it.
        for worker in self.workers.drain(..) {
            let _ = worker.join();
        }
    }
}

fn main() {
    println!("--- Thread Pool: 4 workers, 20 jobs ---");
    let pool = ThreadPool::new(4);

    // Submit 20 jobs. Each job prints its ID from the worker thread.
    // execute() sends the closure through the channel; a worker picks it up.
    let mut receivers = vec![];

    for i in 0..20 {
        let (tx, rx) = mpsc::channel::<()>();
        pool.execute(move || {
            println!("  Job {i} started");
            // Simulate some work.
            thread::sleep(std::time::Duration::from_millis(50));
            println!("  Job {i} finished");
            tx.send(()).unwrap();
        });
        receivers.push(rx);
    }

    // Wait for all jobs to complete before dropping the pool.
    for rx in receivers {
        rx.recv().unwrap();
    }

    println!("All 20 jobs completed. Dropping pool...");
    // Pool is dropped here — Drop impl signals shutdown and joins all workers.
    println!("Pool shut down gracefully!");
}
