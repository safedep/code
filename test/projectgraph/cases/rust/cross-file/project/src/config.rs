use std::fs;

pub struct Config {
    pub path: String,
}

impl Config {
    pub fn load(path: &str) -> Config {
        let text = fs::read_to_string(path).unwrap();
        Config { path: text }
    }

    fn unused(&self) {
        std::process::Command::new("ls").spawn().unwrap();
    }
}
