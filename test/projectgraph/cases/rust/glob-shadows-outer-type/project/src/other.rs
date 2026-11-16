pub struct A;

impl A {
    pub fn new() -> A {
        A
    }

    pub fn run(&self) {
        let _ = std::fs::remove_file("a");
    }
}
