pub struct Item;

impl Item {
    pub fn save(&self) {
        let _ = std::fs::write("top", "x");
    }

    pub fn new() -> Item {
        Item
    }
}

pub mod cache {
    pub struct Item;

    impl Item {
        pub fn save(&self) {
            let _ = std::fs::create_dir("cache");
        }

        pub fn new() -> Item {
            Item
        }
    }

    pub mod deep {
        use crate::store::Item;

        pub fn by_new() {
            let x = Item::new();
            x.save();
        }

        pub fn by_value() {
            let x = Item;
            x.save();
        }
    }
}
