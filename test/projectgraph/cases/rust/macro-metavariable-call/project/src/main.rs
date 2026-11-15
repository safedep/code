macro_rules! call {
    ($f:ident) => {
        $f()
    };
}

fn target() {
    let _ = std::fs::remove_dir("a");
}

fn main() {
    call!(target);
}
