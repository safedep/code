pub struct Local;
pub struct Real;

pub trait Maker {
    fn make() -> Self;
}

impl Local {
    pub fn make() -> Local {
        let _ = std::fs::remove_file("local");
        Local
    }
}

impl Maker for Real {
    fn make() -> Real {
        let _ = std::fs::create_dir("real");
        Real
    }
}

#[allow(non_camel_case_types)]
fn build<Local: Maker>() -> Local {
    Local::make()
}

fn main() {
    let _ = build::<Real>();
}
