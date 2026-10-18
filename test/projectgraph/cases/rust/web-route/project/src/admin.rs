pub fn purge() {
    std::fs::remove_dir_all("/tmp/cache").unwrap();
}
