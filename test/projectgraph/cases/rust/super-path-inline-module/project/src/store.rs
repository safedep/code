pub struct Item;

impl Item {
    pub fn new() -> Item {
        let _ = std::fs::write("top", "x");
        Item
    }

    pub fn save(&self) {
        let _ = std::fs::remove_file("top");
    }
}

pub mod other {
    pub struct Item;

    impl Item {
        pub fn save(&self) {
            let _ = std::fs::remove_dir("other");
        }
    }

    pub fn sib() {
        let i = super::Item::new();
        i.save();
        self::inner::run();
    }

    pub fn made() {
        let j = super::Item {};
        j.save();
    }

    pub fn rooted() {
        let k = crate::store::Item::new();
        k.save();
    }

    pub mod inner {
        pub fn run() {
            let _ = std::fs::create_dir("inner");
        }
    }
}
