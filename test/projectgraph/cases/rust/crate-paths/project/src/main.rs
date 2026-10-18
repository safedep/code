mod store;

fn z() {
    std::process::Command::new("ls");
}

fn main() {
    store::run();
}
