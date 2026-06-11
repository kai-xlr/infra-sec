use criterion::{black_box, criterion_group, criterion_main, Criterion};
use policy_engine::evaluator::Engine;
use policy_engine::policy::Policy;

fn build_engine() -> Engine {
    Engine::new(vec![
        Policy::new("admin", "write"),
        Policy::new("admin", "delete"),
        Policy::new("developer", "write"),
        Policy::new("viewer", "read"),
    ])
}

fn single_evaluation(c: &mut Criterion) {
    let engine = build_engine();

    c.bench_function("single evaluation", |b| {
        b.iter(|| engine.evaluate(black_box("viewer"), black_box("read")))
    });
}

fn multi_evaluation_1000(c: &mut Criterion) {
    let engine = build_engine();

    c.bench_function("1000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..1000 {
                engine.evaluate(black_box("viewer"), black_box("read"));
            }
        })
    });
}

fn multi_evaluation_10000(c: &mut Criterion) {
    let engine = build_engine();

    c.bench_function("10000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..10000 {
                engine.evaluate(black_box("viewer"), black_box("read"));
            }
        })
    });
}

criterion_group!(benches, single_evaluation, multi_evaluation_1000, multi_evaluation_10000);
criterion_main!(benches);
