mod handlers;

fn cleanup(p: &str) {
    let _ = std::fs::remove_file(p);
}

fn dispatch<T>(items: Vec<T>, s: &str) {
    for h in items {
        h.handle(s);
    }
}

fn main() {
    dispatch(vec![handlers::H0], "a");
}
