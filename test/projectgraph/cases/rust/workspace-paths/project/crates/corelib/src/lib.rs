pub mod a;
mod hidden;

pub use hidden::reexported as transmit;
pub use hidden::glob::*;

#[macro_export]
macro_rules! shout {
    ($x:expr) => {
        std::process::abort()
    };
}

pub trait Task {
    fn run(&self) {
        self.step();
    }
    fn step(&self);
}

pub fn top() {
    std::fs::create_dir("top").unwrap();
}
