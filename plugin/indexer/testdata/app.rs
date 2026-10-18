use crate::store::Store;
use serde_yaml;

pub struct App {
    store: Box<dyn Store>,
}

impl App {
    pub fn new(store: Box<dyn Store>) -> Self {
        App { store }
    }

    #[instrument]
    pub fn run(&self, path: &str) -> Result<(), Error> {
        let text: String = std::fs::read_to_string(path)?;
        self.store.save(serde_yaml::from_str(&text).unwrap());
        self.audit();
        println!("{}", helper(path));
        Ok(())
    }

    fn audit(&self) {
        log::info!("ran");
    }
}

impl Runner for App {
    fn go(&self) {
        self.run("a.yaml").unwrap();
    }
}

pub trait Runner {
    fn go(&self);
}

fn helper(path: &str) -> String {
    path.trim().to_string()
}

fn main() {
    App::new(Box::new(crate::store::Memory {})).go();
}
