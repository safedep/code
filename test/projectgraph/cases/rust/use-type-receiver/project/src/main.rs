mod store;

mod a {
    pub struct Item;

    impl Item {
        pub fn new() -> Item {
            Item
        }

        pub fn save(&self) {
            let _ = std::fs::remove_file("a");
        }
    }
}

mod b {
    pub struct Item;

    impl Item {
        pub fn save(&self) {
            let _ = std::fs::remove_dir("b");
        }
    }
}

fn qualified() {
    use crate::b::Item;
    let _b = Item;
    let x = a::Item::new();
    x.save();
}

fn main() {
    store::cache::deep::by_new();
    store::cache::deep::by_value();
    qualified();
}
