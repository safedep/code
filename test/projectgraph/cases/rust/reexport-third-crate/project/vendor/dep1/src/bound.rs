pub use dep2::store::save;

pub fn audit() {
    let _ = std::fs::read("audit");
}
