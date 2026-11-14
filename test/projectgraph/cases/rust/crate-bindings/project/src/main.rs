mod defs;
mod secure;
mod store;

pub use kit as ffi;

fn main() {
    secure::aes::make();
    store::sync();
    defs::run();
}

#[cfg(test)]
mod tests {
    use crate::ffi;
}
