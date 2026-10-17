pub trait Shout {
    fn shout(&self);
}

impl Shout for String {
    fn shout(&self) {
        std::fs::copy("a", "b").unwrap();
    }
}
