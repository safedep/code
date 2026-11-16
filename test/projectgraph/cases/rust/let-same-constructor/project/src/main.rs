mod types;

use types::A;

fn go() {
    let t = A::new();
    drop(t);
    let t = A::new();
    t.run();
}

fn main() {
    go();
    let _ = types::B::new();
}
