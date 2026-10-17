use super::Store;

pub struct DbStore {}

impl Store for DbStore {
    fn save(&self, key: &str) {
        std::process::Command::new("psql").arg(key).spawn().unwrap();
    }
}
