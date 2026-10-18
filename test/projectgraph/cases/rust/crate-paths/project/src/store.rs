fn w() {
    std::fs::remove_file("/x");
}

pub fn run() {
    crate::z();
    super::z();
    self::w();
}
