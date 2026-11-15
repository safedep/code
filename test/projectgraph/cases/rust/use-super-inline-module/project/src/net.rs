pub mod tls;

pub mod client {
    use super::tls;

    pub fn go() {
        tls::open();
    }
}
