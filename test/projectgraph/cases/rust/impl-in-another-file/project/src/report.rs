pub struct Report {
    pub name: String,
}

impl Report {
    pub fn new(name: &str) -> Self {
        Report { name: name.to_string() }
    }

    pub fn publish(&self) {
        self.render();
        self.upload();
    }

    fn render(&self) -> String {
        format!("report {}", self.name)
    }
}
