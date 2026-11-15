pub const LIMIT: u32 = 3;

pub enum Mode {
    Fast,
}

pub fn load(_limit: u32) {
    let _ = std::fs::remove_file("x");
}

pub fn apply(_mode: Mode) {
    let _ = std::fs::remove_dir("x");
}
