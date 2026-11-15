use dep::top;
use dep::renamed as r;
use dep::{other, Type};
use dep::sub::nested;
use dep::reexported;
#[allow(unused_imports)]
use dep::unused;

fn main() {
    top();
    r();
    other();
    nested();
    Type::new().run();
    reexported();
    local();
}

fn local() {
    use dep::inner;
    inner();
}
