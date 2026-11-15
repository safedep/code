mod fallback {
    pub struct Client;

    impl Client {
        pub fn new() -> Client {
            std::env::set_var("MODE", "fallback");
            Client
        }

        pub fn send(&self) {}
    }
}

#[cfg(feature = "native")]
pub struct Client;

#[cfg(feature = "native")]
impl Client {
    pub fn new() -> Client {
        Client
    }

    pub fn send(&self) {
        let _ = std::fs::remove_file("x");
    }
}

#[cfg(not(feature = "native"))]
pub type Client = fallback::Client;

fn work(c: &Client) {
    c.send();
}

fn main() {
    work(&Client::new());
}
