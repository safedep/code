mod stores;

use stores::{db::DbStore, file::FileStore, Store};

fn persist(store: &dyn Store, key: &str) {
    store.save(key);
}

fn flush<S: Store>(store: S) {
    store.flush();
}

fn main() {
    persist(&DbStore {}, "k");
    flush(FileStore {});
}
