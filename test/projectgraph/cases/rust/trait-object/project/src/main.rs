use dep::{Html, Write};

fn main() {
    let w: Box<dyn Write> = Box::new(Html {});
    w.write();
}
