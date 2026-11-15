mod util {
    pub fn wipe(p: &str) {
        let _ = std::fs::remove_file(p);
    }
}

pub mod worker {
    use crate::store::util;

    pub fn go() {
        util::wipe("x");
    }
}

pub fn flush() {
    use crate::store::util;
    util::wipe("y");
}
