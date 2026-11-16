struct Client;

impl Client {
    fn new() -> Client {
        Client
    }

    fn with_retry(self) -> Client {
        self
    }

    fn send(&self) {
        let _ = std::fs::write("a", "b");
    }
}

struct Other;

impl Other {
    fn send(&self) {
        let _ = std::fs::create_dir("c");
    }
}

fn builder() {
    let c = Client::new();
    let c = c.with_retry();
    c.send();
}

fn branch(flag: bool) {
    let c = Client::new();
    if flag {
        let c = Client;
        c.send();
    }
    let _ = c;
}

fn main() {
    builder();
    branch(true);
}
