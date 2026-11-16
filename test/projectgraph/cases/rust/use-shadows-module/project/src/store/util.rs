pub fn wipe(p: &str) {
    let _ = std::fs::remove_dir_all(p);
}
