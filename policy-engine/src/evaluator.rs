use crate::policy::Policy;

pub struct Engine {
    policies: Vec<Policy>,
}

impl Engine {
    pub fn new(policies: Vec<Policy>) -> Self {
        Self { policies }
    }

    pub fn evaluate(&self, role: &str, action: &str) -> bool {
        self.policies.iter().any(|p| p.role == role && p.action == action)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_engine() -> Engine {
        Engine::new(vec![
            Policy::new("admin", "write", "allow"),
            Policy::new("admin", "delete", "allow"),
            Policy::new("developer", "write", "allow"),
            Policy::new("viewer", "read", "allow"),
        ])
    }

    #[test]
    fn viewer_can_read() {
        let engine = test_engine();
        assert!(engine.evaluate("viewer", "read"));
    }

    #[test]
    fn viewer_cannot_write() {
        let engine = test_engine();
        assert!(!engine.evaluate("viewer", "write"));
    }

    #[test]
    fn admin_can_delete() {
        let engine = test_engine();
        assert!(engine.evaluate("admin", "delete"));
    }

    #[test]
    fn unknown_role_denied() {
        let engine = test_engine();
        assert!(!engine.evaluate("unknown", "read"));
    }

    #[test]
    fn unknown_action_denied() {
        let engine = test_engine();
        assert!(!engine.evaluate("admin", "unknown"));
    }
}
