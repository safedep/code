pub struct Raw;

pub struct Store;

impl From<Raw> for Store {
    fn from(_r: Raw) -> Store {
        let _ = std::fs::remove_file("x");
        Store
    }
}

pub fn raw() -> Raw {
    Raw
}
