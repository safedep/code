use super::Store;

pub struct FileStore {}

impl Store for FileStore {
    fn save(&self, key: &str) {
        std::fs::remove_file(key).unwrap();
    }

    fn flush(&self) {
        std::fs::File::open("x").unwrap().sync_all().unwrap();
    }
}
