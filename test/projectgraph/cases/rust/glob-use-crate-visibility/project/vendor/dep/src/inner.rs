pub fn save() {
    let _ = std::fs::remove_dir("b");
}

pub fn load() {
    let _ = std::fs::remove_dir("c");
}
