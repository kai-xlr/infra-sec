use policy_engine::evaluator::Engine;
use policy_engine::policy::{Policy, RoleHierarchy};

#[test]
fn integration_viewer_read() {
    let engine = Engine::new(vec![
        Policy::new("admin", "write", "allow"),
        Policy::new("admin", "delete", "allow"),
        Policy::new("developer", "write", "allow"),
        Policy::new("viewer", "read", "allow"),
    ]);

    let hierarchy = RoleHierarchy::default();

    assert_eq!(engine.evaluate("viewer", "read", &hierarchy), Some(true));
    assert_eq!(engine.evaluate("viewer", "write", &hierarchy), None);
    assert_eq!(engine.evaluate("viewer", "delete", &hierarchy), None);
}

#[test]
fn integration_admin_inherits_all() {
    let engine = Engine::new(vec![
        Policy::new("admin", "write", "allow"),
        Policy::new("admin", "delete", "allow"),
        Policy::new("developer", "write", "allow"),
        Policy::new("viewer", "read", "allow"),
    ]);

    let hierarchy = RoleHierarchy::default();

    // admin inherits viewer read via hierarchy
    assert_eq!(engine.evaluate("admin", "read", &hierarchy), Some(true));
    assert_eq!(engine.evaluate("admin", "write", &hierarchy), Some(true));
    assert_eq!(engine.evaluate("admin", "delete", &hierarchy), Some(true));
}
