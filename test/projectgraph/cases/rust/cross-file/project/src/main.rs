mod config;
mod store;

use crate::config::Config;
use store::{Store, file::FileStore};

fn main() {
    let cfg = Config::load("a.toml");
    let s = FileStore::new(&cfg.path);
    persist(&s, "k");
    println!("{}", helper(2));
}

fn persist(s: &dyn Store, key: &str) {
    s.save(key);
}

fn helper(x: i32) -> i32 {
    x * 2
}
