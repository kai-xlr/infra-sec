#[derive(Debug, Clone)]
pub struct Policy {
    pub role: String,
    pub action: String,
}

impl Policy {
    pub fn new(role: &str, action: &str) -> Self {
        Self {
            role: role.to_string(),
            action: action.to_string(),
        }
    }
}
