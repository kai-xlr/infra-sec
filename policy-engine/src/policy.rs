use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq, Eq)]
pub struct Policy {
    pub role: String,
    pub action: String,
    pub effect: String,
}

impl Policy {
    pub fn new(role: &str, action: &str, effect: &str) -> Self {
        Self {
            role: role.to_string(),
            action: action.to_string(),
            effect: effect.to_string(),
        }
    }

    pub fn from_json(json_str: &str) -> Result<Vec<Policy>, String> {
        let policies: Vec<Policy> = serde_json::from_str(json_str)
            .map_err(|e| format!("Failed to parse JSON: {}", e))?;
        validate_policies(&policies)?;
        Ok(policies)
    }
}

pub(crate) fn validate_policies(policies: &[Policy]) -> Result<(), String> {
    for (index, policy) in policies.iter().enumerate() {
        if policy.role.trim().is_empty() {
            return Err(format!(
                "Validation error at index {}: 'role' cannot be empty",
                index
            ));
        }
        if policy.action.trim().is_empty() {
            return Err(format!(
                "Validation error at index {}: 'action' cannot be empty",
                index
            ));
        }
        if policy.effect.trim().is_empty() {
            return Err(format!(
                "Validation error at index {}: 'effect' cannot be empty",
                index
            ));
        }
        match policy.effect.as_str() {
            "allow" | "deny" => {}
            invalid_effect => {
                return Err(format!(
                    "Validation error at index {}: 'effect' must be 'allow' or 'deny', found '{}'",
                    index, invalid_effect
                ));
            }
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_from_json_valid() {
        let json = r#"[
            {"role": "admin", "action": "delete", "effect": "allow"},
            {"role": "viewer", "action": "read", "effect": "allow"}
        ]"#;

        let policies = Policy::from_json(json).unwrap();
        assert_eq!(policies.len(), 2);
        assert_eq!(policies[0], Policy::new("admin", "delete", "allow"));
        assert_eq!(policies[1], Policy::new("viewer", "read", "allow"));
    }

    #[test]
    fn test_from_json_invalid_json() {
        let json = r#"not valid json"#;
        let result = Policy::from_json(json);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Failed to parse JSON"));
    }

    #[test]
    fn test_from_json_invalid_effect() {
        let json = r#"[
            {"role": "admin", "action": "delete", "effect": "maybe"}
        ]"#;
        let result = Policy::from_json(json);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .contains("must be 'allow' or 'deny', found 'maybe'"));
    }

    #[test]
    fn test_from_json_empty_role() {
        let json = r#"[
            {"role": "", "action": "write", "effect": "allow"}
        ]"#;
        let result = Policy::from_json(json);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .contains("'role' cannot be empty"));
    }

    #[test]
    fn test_from_json_missing_field() {
        let json = r#"[
            {"role": "admin", "action": "delete"}
        ]"#;
        let result = Policy::from_json(json);
        assert!(result.is_err());
    }

    #[test]
    fn test_json_round_trip() {
        let original = vec![
            Policy::new("admin", "delete", "allow"),
            Policy::new("viewer", "read", "allow"),
        ];

        let serialized = serde_json::to_string(&original).unwrap();
        let deserialized: Vec<Policy> = serde_json::from_str(&serialized).unwrap();

        assert_eq!(original, deserialized);
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RoleHierarchy {
    pub map: HashMap<String, Vec<String>>,
}

impl RoleHierarchy {
    pub fn new(map: HashMap<String, Vec<String>>) -> Self {
        Self { map }
    }
}

impl Default for RoleHierarchy {
    fn default() -> Self {
        let mut map = HashMap::new();

        map.insert(
            "admin".to_string(),
            vec!["developer".to_string(), "viewer".to_string()],
        );
        map.insert("developer".to_string(), vec!["viewer".to_string()]);
        map.insert("viewer".to_string(), vec![]);

        Self { map }
    }
}
