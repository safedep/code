mod store;

fn main() {
    store::cache::deep::by_new();
    store::cache::deep::by_value();
}
