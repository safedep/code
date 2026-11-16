#[macro_use]
extern crate dep;
#[macro_use]
mod macros;
mod job;

fn main() {
    job::go();
}

pub fn flush() {
    let _ = std::fs::remove_dir("a");
}
