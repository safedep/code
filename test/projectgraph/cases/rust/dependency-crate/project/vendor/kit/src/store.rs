use crate::Sink;

pub struct Store {
    path: String,
}

impl Store {
    pub fn open() -> Result<Store, String> {
        Ok(Store { path: String::new() })
    }

    pub fn save(&self) {
        std::fs::write(&self.path, "data").unwrap();
    }
}

impl Sink for Store {
    fn flush(&self) {
        std::fs::remove_file(&self.path).unwrap();
    }
}
