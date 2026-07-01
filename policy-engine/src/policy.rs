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
