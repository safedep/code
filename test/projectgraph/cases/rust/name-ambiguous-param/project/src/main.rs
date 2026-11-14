mod handlers;

fn cleanup(p: &str) {
    let _ = std::fs::remove_file(p);
}

fn pick() -> fn(&str) {
    cleanup
}

fn dispatch(handle: fn(&str), s: &str) {
    handle(s)
}

fn main() {
    dispatch(pick(), "a");
}
