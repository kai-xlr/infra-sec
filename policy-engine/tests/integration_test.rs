use policy_engine::evaluator::Engine;
use policy_engine::policy::Policy;

#[test]
fn integration_viewer_read() {
    let engine = Engine::new(vec![
        Policy::new("admin", "write"),
        Policy::new("admin", "delete"),
        Policy::new("developer", "write"),
        Policy::new("viewer", "read"),
    ]);

    assert!(engine.evaluate("viewer", "read"));
    assert!(!engine.evaluate("viewer", "write"));
    assert!(!engine.evaluate("viewer", "delete"));
}

#[test]
fn integration_admin_all() {
    let engine = Engine::new(vec![
        Policy::new("admin", "write"),
        Policy::new("admin", "delete"),
        Policy::new("developer", "write"),
        Policy::new("viewer", "read"),
    ]);

    // admin has no explicit read in this policy set
    // so this correctly tests that only explicit rules match
    assert!(!engine.evaluate("admin", "read"));
    assert!(engine.evaluate("admin", "write"));
    assert!(engine.evaluate("admin", "delete"));
}
