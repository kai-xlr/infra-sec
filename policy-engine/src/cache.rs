use std::collections::HashMap;
use std::time::{Duration, Instant};

pub struct PolicyCache {
    cache: HashMap<(String, String), CacheEntry>,
    ttl: Duration,
}

struct CacheEntry {
    decision: bool,
    expires_at: Instant,
}

impl PolicyCache {
    pub fn new(ttl: Duration) -> Self {
        Self {
            cache: HashMap::new(),
            ttl,
        }
    }

    pub fn get(&mut self, role: &str, action: &str) -> Option<bool> {
        self.evict_expired();

        let key = (role.to_string(), action.to_string());
        self.cache.get(&key).map(|entry| entry.decision)
    }

    pub fn set(&mut self, role: &str, action: &str, decision: bool) {
        let key = (role.to_string(), action.to_string());
        let expires_at = Instant::now() + self.ttl;

        self.cache.insert(
            key,
            CacheEntry {
                decision,
                expires_at,
            },
        );
    }

    fn evict_expired(&mut self) {
        let now = Instant::now();
        self.cache.retain(|_, entry| entry.expires_at > now);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cache_hit_and_miss() {
        let mut cache = PolicyCache::new(Duration::from_secs(10));

        assert_eq!(cache.get("admin", "delete"), None);

        cache.set("admin", "delete", true);
        assert_eq!(cache.get("admin", "delete"), Some(true));
    }

    #[test]
    fn test_cache_overwrite() {
        let mut cache = PolicyCache::new(Duration::from_secs(10));

        cache.set("user", "write", true);
        assert_eq!(cache.get("user", "write"), Some(true));

        cache.set("user", "write", false);
        assert_eq!(cache.get("user", "write"), Some(false));
    }

    #[test]
    fn test_cache_ttl_expiry() {
        let mut cache = PolicyCache::new(Duration::from_millis(10));

        cache.set("guest", "read", true);
        assert_eq!(cache.get("guest", "read"), Some(true));

        std::thread::sleep(Duration::from_millis(15));

        assert_eq!(cache.get("guest", "read"), None);
    }
}
