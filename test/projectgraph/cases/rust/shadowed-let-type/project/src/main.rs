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

fn go() {
    let t = Disk;
    t.save();
    let t = Cloud;
    t.save();
}

fn main() {
    go();
}
