extern crate aes;

pub fn make() {
    use self::aes::Cipher;

    crate::secure::kit::flush();
    Cipher::new();
}
