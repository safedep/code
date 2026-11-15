pub struct Item;

impl Item {
    pub fn save(&self) {
        let _ = std::fs::write("top", "x");
    }
}

pub mod cache {
    pub struct Item;

    impl Item {
        pub fn save(&self) {
            let _ = std::fs::create_dir("cache");
        }
    }

    pub mod deep {
        use crate::store::Item;

        pub struct Holder {
            pub item: Item,
        }

        pub fn make() -> Item {
            Item
        }

        pub fn by_result() {
            make().save();
        }

        pub fn by_field() {
            let h = Holder { item: Item };
            h.item.save();
        }
    }
}
