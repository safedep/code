pub struct A;
pub struct B;

impl A {
    pub fn new() -> A {
        A
    }

    pub fn run(&self) {
        let _ = std::fs::remove_file("a");
    }
}

impl B {
    pub fn new() -> B {
        B
    }

    pub fn run(&self) {
        let _ = std::fs::remove_dir("b");
    }
}
