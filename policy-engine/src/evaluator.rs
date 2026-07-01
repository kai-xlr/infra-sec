use std::collections::HashMap;

use crate::policy::{Policy, RoleHierarchy};

pub struct Engine {
    policies: HashMap<(String, String), bool>,
    raw_policies: Vec<Policy>,
}

impl Engine {
    pub fn new(policies: Vec<Policy>) -> Self {
        let mut policy_map = HashMap::with_capacity(policies.len());

        for policy in &policies {
            let is_allowed = policy.effect == "allow";
            policy_map.insert((policy.role.clone(), policy.action.clone()), is_allowed);
        }

        Self {
            policies: policy_map,
            raw_policies: policies,
        }
    }

    pub fn evaluate(&self, role: &str, action: &str, hierarchy: &RoleHierarchy) -> Option<bool> {
        if let Some(true) = self
            .policies
            .get(&(role.to_string(), action.to_string()))
            .copied()
        {
            return Some(true);
        }

        if let Some(inherited_roles) = hierarchy.map.get(role) {
            for inherited_role in inherited_roles {
                if let Some(true) = self
                    .policies
                    .get(&(inherited_role.to_string(), action.to_string()))
                    .copied()
                {
                    return Some(true);
                }
            }
        }

        None
    }

    pub fn evaluate_linear(
        &self,
        role: &str,
        action: &str,
        hierarchy: &RoleHierarchy,
    ) -> Option<bool> {
        if let Some(p) = self
            .raw_policies
            .iter()
            .find(|p| p.role == role && p.action == action)
        {
            if p.effect == "allow" {
                return Some(true);
            }
        }

        if let Some(inherited_roles) = hierarchy.map.get(role) {
            for inherited_role in inherited_roles {
                if let Some(p) = self
                    .raw_policies
                    .iter()
                    .find(|p| p.role == *inherited_role && p.action == action)
                {
                    if p.effect == "allow" {
                        return Some(true);
                    }
                }
            }
        }

        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_engine() -> Engine {
        Engine::new(vec![
            Policy::new("admin", "delete", "allow"),
            Policy::new("developer", "write", "allow"),
            Policy::new("viewer", "read", "allow"),
        ])
    }

    #[test]
    fn viewer_can_read() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();
        assert_eq!(engine.evaluate("viewer", "read", &hierarchy), Some(true));
        assert_eq!(engine.evaluate_linear("viewer", "read", &hierarchy), Some(true));
    }

    #[test]
    fn viewer_cannot_write() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();
        assert_eq!(engine.evaluate("viewer", "write", &hierarchy), None);
        assert_eq!(engine.evaluate_linear("viewer", "write", &hierarchy), None);
    }

    #[test]
    fn admin_can_delete() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();
        assert_eq!(engine.evaluate("admin", "delete", &hierarchy), Some(true));
        assert_eq!(engine.evaluate_linear("admin", "delete", &hierarchy), Some(true));
    }

    #[test]
    fn admin_inherits_viewer_permissions() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();

        assert_eq!(engine.evaluate("admin", "read", &hierarchy), Some(true));
        assert_eq!(engine.evaluate_linear("admin", "read", &hierarchy), Some(true));
    }

    #[test]
    fn developer_inherits_viewer_permissions() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();

        assert_eq!(engine.evaluate("developer", "read", &hierarchy), Some(true));
        assert_eq!(engine.evaluate_linear("developer", "read", &hierarchy), Some(true));
    }

    #[test]
    fn unknown_role_denied() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();
        assert_eq!(engine.evaluate("anonymous", "read", &hierarchy), None);
        assert_eq!(engine.evaluate_linear("anonymous", "read", &hierarchy), None);
    }

    #[test]
    fn unknown_action_denied() {
        let engine = test_engine();
        let hierarchy = RoleHierarchy::default();
        assert_eq!(engine.evaluate("admin", "unknown", &hierarchy), None);
        assert_eq!(engine.evaluate_linear("admin", "unknown", &hierarchy), None);
    }
}
