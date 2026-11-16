pub mod tls {
    pub mod conn {
        pub fn open() {
            let _ = std::fs::read("top");
        }
    }
}

pub mod client {
    pub mod tls {
        pub mod conn {
            pub fn open() {
                let _ = std::fs::create_dir("client");
            }
        }
    }

    use self::tls::conn;

    pub fn go() {
        conn::open();
    }
}
