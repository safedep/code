pub fn send() {
    std::fs::write("disk", "x").unwrap();
}

pub fn run() {
    self::flush();
}

fn flush() {
    std::fs::File::create("flush").unwrap();
}

mod inner {
    pub fn flush() {
        std::fs::remove_dir_all("inner").unwrap();
    }
}
