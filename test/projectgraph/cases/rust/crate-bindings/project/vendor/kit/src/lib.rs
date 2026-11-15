pub struct Store {
    path: String,
}

impl Store {
    pub fn open() -> Store {
        Store { path: String::new() }
    }

    pub fn save(&self) {
        std::fs::write(&self.path, "data").unwrap();
    }
}

pub fn flush() {}

pub fn reset() {}

pub fn unused() {}
