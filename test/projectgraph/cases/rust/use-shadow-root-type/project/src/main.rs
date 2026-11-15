mod other;

pub struct A;

impl A {
    pub fn new() -> A {
        A
    }

    pub fn run(&self) {
        let _ = std::fs::remove_dir("a");
    }
}

mod deep {
    use crate::other::A;

    pub struct Holder {
        pub a: A,
    }

    pub fn by_field(h: &Holder) {
        h.a.run();
    }

    pub fn by_param(x: A) {
        x.run();
    }

    pub fn by_new() {
        let x = A::new();
        x.run();
    }
}

fn by_block() {
    use crate::other::A;
    A::new().run();
}

fn main() {
    deep::by_field(&deep::Holder { a: other::A::new() });
    deep::by_param(other::A::new());
    deep::by_new();
    by_block();
    let _ = A::new();
}
