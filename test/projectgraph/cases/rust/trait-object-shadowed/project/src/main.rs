use dep::{Html, Write};

fn main() {
    print(Box::new(Html {}));
}

fn print(w: Box<dyn Write>) {
    let w = dep::plain();
    w.write();
}
