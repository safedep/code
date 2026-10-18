pub mod db;
pub mod file;

pub trait Store {
    fn save(&self, key: &str);

    fn flush(&self) {
        std::io::stdout().flush_all();
    }
}
