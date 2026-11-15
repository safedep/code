pub struct Item;

impl Item {
    pub fn save(&self) {
        let _ = std::fs::write("top", "x");
    }
}

pub struct Holder {
    pub item: Item,
}

pub mod cache {
    pub struct Item;

    impl Item {
        pub fn save(&self) {
            let _ = std::fs::create_dir("cache");
        }
    }

    pub struct Holder {
        pub item: Item,
    }

    pub fn make() -> Item {
        Item
    }

    pub fn go() {
        let h = Holder { item: Item };
        h.item.save();
        make().save();
    }
}
