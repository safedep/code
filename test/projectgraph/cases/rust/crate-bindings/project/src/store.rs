use super::ffi;

pub fn sync() {
    ffi::reset();
    crate::ffi::Store::open();
}
