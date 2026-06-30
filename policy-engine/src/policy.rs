use serde::{Deserialize, Serialize};

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
