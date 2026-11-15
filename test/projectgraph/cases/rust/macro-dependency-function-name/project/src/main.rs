#[macro_use]
extern crate dep;
mod job;
mod util;

fn main() {
    util::log(1);
    job::go();
}
