use criterion::{black_box, criterion_group, criterion_main, Criterion};
use policy_engine::evaluator::Engine;
use policy_engine::policy::{Policy, RoleHierarchy};

fn build_small_engine() -> Engine {
    Engine::new(vec![
        Policy::new("admin", "write", "allow"),
        Policy::new("admin", "delete", "allow"),
        Policy::new("developer", "write", "allow"),
        Policy::new("viewer", "read", "allow"),
    ])
}

fn build_large_engine() -> Engine {
    let mut policies = Vec::with_capacity(150);
    for i in 0..100 {
        policies.push(Policy::new(
            &format!("role_{}", i),
            "action_a",
            if i % 2 == 0 { "allow" } else { "deny" },
        ));
    }
    for i in 0..50 {
        policies.push(Policy::new(
            &format!("role_{}", i),
            "action_b",
            "allow",
        ));
    }
    Engine::new(policies)
}

fn single_evaluation(c: &mut Criterion) {
    let engine = build_small_engine();
    let hierarchy = RoleHierarchy::default();

    c.bench_function("HashMap single evaluation", |b| {
        b.iter(|| engine.evaluate(black_box("viewer"), black_box("read"), black_box(&hierarchy)))
    });

    c.bench_function("linear single evaluation", |b| {
        b.iter(|| engine.evaluate_linear(black_box("viewer"), black_box("read"), black_box(&hierarchy)))
    });
}

fn multi_evaluation_1000(c: &mut Criterion) {
    let engine = build_small_engine();
    let hierarchy = RoleHierarchy::default();

    c.bench_function("HashMap 1000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..1000 {
                engine.evaluate(black_box("viewer"), black_box("read"), black_box(&hierarchy));
            }
        })
    });

    c.bench_function("linear 1000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..1000 {
                engine.evaluate_linear(black_box("viewer"), black_box("read"), black_box(&hierarchy));
            }
        })
    });
}

fn multi_evaluation_10000(c: &mut Criterion) {
    let engine = build_small_engine();
    let hierarchy = RoleHierarchy::default();

    c.bench_function("HashMap 10000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..10000 {
                engine.evaluate(black_box("viewer"), black_box("read"), black_box(&hierarchy));
            }
        })
    });

    c.bench_function("linear 10000 evaluations", |b| {
        b.iter(|| {
            for _ in 0..10000 {
                engine.evaluate_linear(black_box("viewer"), black_box("read"), black_box(&hierarchy));
            }
        })
    });
}

fn large_engine_bench(c: &mut Criterion) {
    let engine = build_large_engine();
    let hierarchy = RoleHierarchy::default();

    c.bench_function("HashMap large engine (150 policies)", |b| {
        b.iter(|| engine.evaluate(black_box("role_99"), black_box("action_a"), black_box(&hierarchy)))
    });

    c.bench_function("linear large engine (150 policies)", |b| {
        b.iter(|| engine.evaluate_linear(black_box("role_99"), black_box("action_a"), black_box(&hierarchy)))
    });
}

criterion_group!(
    benches,
    single_evaluation,
    multi_evaluation_1000,
    multi_evaluation_10000,
    large_engine_bench,
);
criterion_main!(benches);
