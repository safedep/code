use dep::html::*;

mod inner {
    use other::*;

    pub fn go() {
        render();
    }
}

fn main() {
    inner::go();
    page();
}
