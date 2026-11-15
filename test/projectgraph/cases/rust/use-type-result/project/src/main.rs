mod store;

fn main() {
    store::cache::deep::by_result();
    store::cache::deep::by_field();
}
