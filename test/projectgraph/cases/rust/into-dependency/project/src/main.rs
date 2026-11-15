fn dispatch<T, U: From<T>>(items: Vec<T>) -> Vec<U> {
    let mut out = Vec::new();
    for h in items {
        out.push(h.into());
    }
    out
}

fn main() {
    let _d: Vec<kit::Store> = dispatch(vec![kit::raw()]);
}
