pub fn run() {
    dep::new_thing();
}

pub enum Mode {
    Fast,
}

impl Mode {
    pub fn start() {
        dep::load_thing();
    }
}
