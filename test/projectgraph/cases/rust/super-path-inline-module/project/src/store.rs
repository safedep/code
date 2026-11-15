pub struct Item;

impl Item {
    pub fn new() -> Item {
        let _ = std::fs::write("top", "x");
        Item
    }
}

pub mod other {
    pub fn sib() {
        super::Item::new();
        self::inner::run();
    }

    pub mod inner {
        pub fn run() {
            let _ = std::fs::create_dir("inner");
        }
    }
}
