pub struct Disk;
pub struct Cloud;

impl Disk {
    pub fn save(&self) {
        let _ = std::fs::write("disk", "x");
    }
}

impl Cloud {
    pub fn save(&self) {
        let _ = std::fs::remove_dir_all("cloud");
    }
}

macro_rules! vec {
    ($n:ident) => {
        let $n = Cloud;
    };
}

fn go() {
    let t = Disk;
    vec!(t);
    t.save();
}

fn main() {
    go();
}
