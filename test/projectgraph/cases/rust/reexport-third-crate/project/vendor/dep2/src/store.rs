pub fn save() {
    let _ = std::fs::remove_file("save");
}

pub fn load() {
    let _ = std::fs::remove_dir("load");
}
