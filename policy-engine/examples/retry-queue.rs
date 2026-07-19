// Concept: Retry Queue with Exponential Backoff
//
// This example demonstrates a worker queue that retries failed jobs with
// exponentially increasing delays, moving permanently failed jobs to a
// dead letter queue for later inspection.
//
// Key concepts:
//   Retry queue — Accepts jobs that may fail. On failure, re-queues the
//                 job with a delay that grows exponentially per attempt.
//   Exponential backoff — Delay = base_delay * 2^attempt. Gives transient
//                 failures time to resolve (e.g., network blip, locked DB).
//                 Common in auth token refresh, API calls, distributed systems.
//   Dead letter queue (DLQ) — After max_retries failures, the job is moved
//                 to a DLQ instead of being retried. This prevents infinite
//                 retries and provides visibility into persistent failures.
//   vs jitter — Adding random jitter to backoff prevents thundering herd
//               (all clients retrying at the same time). Our example uses
//               pure exponential; production systems add jitter.
//   vs constant backoff — Constant delay between retries is simpler but
//               less effective: too aggressive for long outages, too slow
//               for short blinks. Exponential adapts naturally.
//   When to retry in auth/policy systems:
//     - Token refresh failures (transient network)
//     - Policy engine unavailable (brief downtime)
//     - Database lock contention (transient)
//     Don't retry: auth failures (wrong password), permission denied (403)
//   Observability — Tracking retry counts and DLQ size is critical for
//                   production. A growing DLQ signals systemic issues.

use std::collections::VecDeque;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use std::cmp;

#[derive(Debug, Clone)]
struct Job {
    id: usize,
    name: String,
}

#[derive(Debug, Clone)]
struct DeadLetterJob {
    job: Job,
    attempts: u32,
    last_error: String,
}

struct RetryQueue {
    max_retries: u32,
    base_delay: Duration,
    dead_letters: Arc<Mutex<Vec<DeadLetterJob>>>,
}

impl RetryQueue {
    fn new(max_retries: u32, base_delay: Duration) -> Self {
        RetryQueue {
            max_retries,
            base_delay,
            dead_letters: Arc::new(Mutex::new(Vec::new())),
        }
    }

    fn run<F>(&self, jobs: Vec<Job>, job_fn: F) -> RetryReport
    where
        F: Fn(&Job) -> Result<(), String> + Send + Sync + 'static,
    {
        let job_fn = Arc::new(job_fn);
        let mut queue: VecDeque<(Job, u32)> = jobs.into_iter().map(|j| (j, 0)).collect();
        let mut succeeded = 0u32;
        let mut failed_total = 0u32;
        let mut dead_letters = 0u32;

        while let Some((job, attempt)) = queue.pop_front() {
            match job_fn(&job) {
                Ok(()) => {
                    println!(
                        "  Job {} '{}' succeeded on attempt {}",
                        job.id, job.name, attempt + 1
                    );
                    succeeded += 1;
                }
                Err(e) => {
                    failed_total += 1;
                    if attempt + 1 >= self.max_retries {
                        println!(
                            "  Job {} '{}' moved to DLQ after {} attempts: {e}",
                            job.id,
                            job.name,
                            attempt + 1
                        );
                        self.dead_letters.lock().unwrap().push(DeadLetterJob {
                            job,
                            attempts: attempt + 1,
                            last_error: e,
                        });
                        dead_letters += 1;
                    } else {
                        // Exponential backoff: base_delay * 2^attempt
                        // Capped to avoid overflow on large attempt counts.
                        let exp = cmp::min(attempt, 31);
                        let delay = self.base_delay * 2u32.pow(exp);
                        println!(
                            "  Job {} '{}' failed (attempt {}): {e}. Retrying in {:?}...",
                            job.id, job.name, attempt + 1, delay
                        );
                        // In a real system, we'd sleep here. For the example,
                        // we skip the sleep to keep the demo fast.
                        queue.push_back((job, attempt + 1));
                    }
                }
            }
        }

        RetryReport {
            succeeded,
            failed: failed_total,
            dead_letters,
        }
    }

    fn drain_dead_letters(&self) -> Vec<DeadLetterJob> {
        std::mem::take(&mut *self.dead_letters.lock().unwrap())
    }
}

#[derive(Debug)]
struct RetryReport {
    succeeded: u32,
    failed: u32,
    dead_letters: u32,
}

impl std::fmt::Display for RetryReport {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "succeeded={}, retried_failures={}, dead_letters={}",
            self.succeeded, self.failed, self.dead_letters
        )
    }
}

fn main() {
    println!("--- Retry Queue: 10 jobs, 50% failure rate ---");
    let queue = RetryQueue::new(3, Duration::from_millis(100));

    // Create 10 jobs. Job IDs 0,2,4,6,8 fail (even); 1,3,5,7,9 succeed (odd).
    // This gives exactly 50% failure rate.
    let jobs: Vec<Job> = (0..10)
        .map(|i| Job {
            id: i,
            name: format!("job-{i}"),
        })
        .collect();

    let report = queue.run(jobs, |job| {
        if job.id % 2 == 0 {
            Err(format!("simulated failure for job {}", job.id))
        } else {
            Ok(())
        }
    });

    println!("\nReport: {report}");

    // Verify dead letter count matches jobs that exhausted all retries.
    // Jobs fail on every attempt, so after 3 retries they hit the DLQ.
    let dead_letters = queue.drain_dead_letters();
    println!("Dead letter queue has {} jobs:", dead_letters.len());
    for dl in &dead_letters {
        println!(
            "  - Job {} '{}': {} attempts, last error: {}",
            dl.job.id, dl.job.name, dl.attempts, dl.last_error
        );
    }

    assert_eq!(report.succeeded, 5, "should have 5 succeeded jobs");
    assert_eq!(
        report.dead_letters, 5,
        "should have 5 dead letter jobs"
    );
    println!("\nVerification passed!");
}
