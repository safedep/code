mod other;

mod m {
    pub struct A;

    impl A {
        pub fn new() -> A {
            A
        }

        pub fn run(&self) {
            let _ = std::fs::remove_dir("a");
        }
    }

    pub mod n {
        use crate::other::*;

        pub fn f() {
            let a = A::new();
            a.run();
        }
    }
}

fn main() {
    m::n::f();
}
