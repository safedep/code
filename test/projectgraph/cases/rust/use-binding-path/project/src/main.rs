mod a;
mod b;
mod c;
mod deep;

pub use dep as ffi;

fn main() {
    a::go();
    b::go();
    c::go();
}
