pub fn cleanup(p: &str) {
    let _ = std::fs::remove_file(p);
}
