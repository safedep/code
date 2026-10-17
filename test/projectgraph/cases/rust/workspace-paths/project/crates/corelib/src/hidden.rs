pub fn reexported() {
    std::fs::remove_file("reexported").unwrap();
}

pub mod glob {
    pub fn globbed() {
        std::env::set_var("glob", "1");
    }
}
