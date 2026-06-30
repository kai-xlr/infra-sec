use std::fs;

use crate::policy::Policy;

fn validate_policies(policies: &[Policy]) -> Result<(), String> {
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

pub fn parse_policy_file(path: &str) -> Result<Vec<Policy>, String> {
    let contents =
        fs::read_to_string(path).map_err(|e| format!("Failed to read file at '{}': {}", path, e))?;

    let policies: Vec<Policy> =
        serde_json::from_str(&contents).map_err(|e| format!("Failed to parse JSON: {}", e))?;

    validate_policies(&policies)?;

    Ok(policies)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_temp_json(filename: &str, content: &str) -> String {
        let path = format!("./target/{}", filename);
        fs::write(&path, content).expect("Failed to write temp test file");
        path
    }

    #[test]
    fn test_parse_valid_policies() {
        let json_data = r#"[
            {"role": "admin", "action": "delete", "effect": "allow"},
            {"role": "user", "action": "read", "effect": "deny"}
        ]"#;

        let path = create_temp_json("valid_policy.json", json_data);

        let result = parse_policy_file(&path);
        assert!(result.is_ok());

        let policies = result.unwrap();
        assert_eq!(policies.len(), 2);
        assert_eq!(policies[0].role, "admin");
        assert_eq!(policies[0].action, "delete");
        assert_eq!(policies[0].effect, "allow");
        assert_eq!(policies[1].role, "user");
        assert_eq!(policies[1].action, "read");
        assert_eq!(policies[1].effect, "deny");

        let _ = fs::remove_file(path);
    }

    #[test]
    fn test_validation_empty_role() {
        let json_data = r#"[
            {"role": "   ", "action": "write", "effect": "allow"}
        ]"#;

        let path = create_temp_json("invalid_role.json", json_data);

        let result = parse_policy_file(&path);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .contains("Validation error at index 0: 'role' cannot be empty"));

        let _ = fs::remove_file(path);
    }

    #[test]
    fn test_validation_invalid_effect() {
        let json_data = r#"[
            {"role": "manager", "action": "approve", "effect": "maybe"}
        ]"#;

        let path = create_temp_json("invalid_effect.json", json_data);

        let result = parse_policy_file(&path);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .contains("must be 'allow' or 'deny', found 'maybe'"));

        let _ = fs::remove_file(path);
    }

    #[test]
    fn test_missing_file() {
        let result = parse_policy_file("this_file_does_not_exist.json");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Failed to read file"));
    }
}
