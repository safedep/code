use super::Store;

pub struct FileStore {
    path: String,
}

impl FileStore {
    pub fn new(path: &str) -> Self {
        FileStore { path: path.to_string() }
    }
}

impl Store for FileStore {
    fn save(&self, key: &str) {
        std::fs::write(&self.path, key).unwrap();
    }
}
