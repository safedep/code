mod convert;

fn cleanup(p: &str) {
    let _ = std::fs::remove_file(p);
}

fn dispatch<T, U: From<T>>(items: Vec<T>) -> Vec<U> {
    let mut out = Vec::new();
    for h in items {
        out.push(h.into());
    }
    out
}

fn main() {
    let _d: Vec<convert::D0> = dispatch(vec![convert::In]);
}
