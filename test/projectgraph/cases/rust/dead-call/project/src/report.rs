pub fn print(name: &str) {
    println!("{}", name);
}

pub fn upload(name: &str) {
    std::process::Command::new("scp").arg(name).spawn().unwrap();
}
