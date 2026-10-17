mod net;
mod disk;
mod shapes;

use shapes::{Circle, Square};

fn send() {
    std::process::exit(1);
}

// std::fs::remove_file("comment") is in a comment
fn main() {
    net::send();
    let send = || 1;
    send();
    let c = Circle {};
    c.area();
    Square::area_of(2);
    let msg = "std::fs::remove_dir_all(\"x\")";
    println!("{}", msg);
    disk::run();
}
