mod flush;

use kit::Store;

fn main() {
    let store = Store::open().unwrap();
    store.save();
    flush::all(&store);
}
