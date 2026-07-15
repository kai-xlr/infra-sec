// Concept: mpsc Producer-Consumer Queue
//
// This example demonstrates the producer-consumer pattern using
// std::sync::mpsc, showing ownership transfer between threads.
//
// Key concepts:
//   mpsc — Multi-Producer, Single-Consumer channel.
//          Ownership of the sent value is *transferred* to the channel.
//          The sender moves the value in; the receiver moves it out.
//          After send(), the original thread can no longer use the value.
//   Sender — Cloning a Sender creates a new handle to the same channel.
//            The channel stays alive as long as at least one Sender exists.
//   Receiver — There is exactly one Receiver. It cannot be cloned.
//              When all Senders are dropped, recv() returns Err (channel closed).
//   Send/Sync — mpsc requires values to be Send (transfer ownership across
//               threads). Channel internals use Mutex/Condvar internally,
//               which is why Sender is Send + Sync but Receiver is only Send.
//   Typed messages — Using a custom enum lets the consumer pattern-match
//                    on message variants, enabling control flow (e.g., stop).
//   Ownership transfer — When you call tx.send(value), the value is moved.
//                         The producer thread no longer owns it. This is a
//                         fundamental Rust guarantee enforced at compile time.
//   Why Send and Sync matter — mpsc channels require the contained type to
//               implement Send (can transfer ownership to another thread).
//               Receiver is !Sync because it holds internal mutable state;
//               it cannot be shared across threads without synchronization
//               (e.g., wrapping in a Mutex).

use std::sync::mpsc;
use std::thread;

enum Command {
    Increment(u64),
    Reset,
    Stop,
}

fn main() {
    // ----- Basic: 1 producer, 1 consumer -----
    let (tx, rx) = mpsc::channel::<Command>();

    let producer = thread::spawn(move || {
        for i in 1..=1000 {
            // Each send() *moves* the Command into the channel.
            // After this line, the producer no longer owns the value.
            tx.send(Command::Increment(i)).unwrap();
        }
        tx.send(Command::Stop).unwrap();
        // tx dropped here — all senders gone, channel closes.
    });

    let consumer = thread::spawn(move || {
        let mut sum: u64 = 0;
        // `for cmd in rx` iterates until the channel closes (all senders dropped).
        // Each cmd is *received* via ownership transfer from the channel.
        for cmd in rx {
            match cmd {
                Command::Increment(n) => sum += n,
                Command::Reset => sum = 0,
                Command::Stop => break,
            }
        }
        sum
    });

    producer.join().unwrap();
    let sum = consumer.join().unwrap();
    println!("Sum: {sum}");

    // ----- Demonstrating Reset -----
    let (tx, rx) = mpsc::channel::<Command>();

    let producer = thread::spawn(move || {
        for i in 1..=100 {
            tx.send(Command::Increment(i)).unwrap();
        }
        tx.send(Command::Reset).unwrap();
        for i in 1..=50 {
            tx.send(Command::Increment(i)).unwrap();
        }
        tx.send(Command::Stop).unwrap();
    });

    let consumer = thread::spawn(move || {
        let mut sum: u64 = 0;
        for cmd in rx {
            match cmd {
                Command::Increment(n) => sum += n,
                Command::Reset => {
                    println!("  Reset triggered! Previous sum: {sum}");
                    sum = 0;
                }
                Command::Stop => break,
            }
        }
        sum
    });

    producer.join().unwrap();
    let sum = consumer.join().unwrap();
    println!("After reset + more work: {sum}");

    // ----- Sender dropped without Stop -----
    // When all Senders are dropped, the channel closes.
    // The consumer's `for` loop ends naturally (recv returns Err).
    let (tx, rx) = mpsc::channel::<Command>();

    thread::spawn(move || {
        tx.send(Command::Increment(42)).unwrap();
        // tx dropped here — no Stop sent, channel closes.
    });

    let collected: Vec<u64> = rx
        .into_iter()
        .filter_map(|cmd| match cmd {
            Command::Increment(n) => Some(n),
            _ => None,
        })
        .collect();
    println!("No Stop, sender dropped: {collected:?}");

    // ----- Multiple producers, single consumer -----
    let (tx, rx) = mpsc::channel::<Command>();
    let mut producers = vec![];

    for id in 0..4 {
        let tx = tx.clone();
        producers.push(thread::spawn(move || {
            for i in 1..=250 {
                tx.send(Command::Increment((id * 250 + i) as u64)).unwrap();
            }
            // Each clone's tx is dropped here — but original tx still alive.
        }));
    }

    // Drop the original sender. Now only the cloned senders remain.
    // When all clones finish and drop, channel closes.
    drop(tx);

    let multi_sum: u64 = rx
        .into_iter()
        .filter_map(|cmd| match cmd {
            Command::Increment(n) => Some(n),
            _ => None,
        })
        .sum();
    println!("Multi-producer sum: {multi_sum}");

    for h in producers {
        h.join().unwrap();
    }
}
