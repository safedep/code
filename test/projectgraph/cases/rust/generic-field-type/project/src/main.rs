pub struct T;

impl T {
    pub fn save(&self) {
        let _ = std::fs::write("t", "x");
    }
}

pub trait Save {
    fn save(&self);
}

pub struct Disk;
pub struct Cloud;

impl Save for Disk {
    fn save(&self) {
        let _ = std::fs::remove_file("disk");
    }
}

impl Save for Cloud {
    fn save(&self) {
        let _ = std::fs::create_dir("cloud");
    }
}

pub struct Holder<T> {
    pub item: T,
}

pub struct Wrap<T>(T);

impl<T: Save> Wrap<T> {
    pub fn get(&self) -> &T {
        &self.0
    }
}

fn by_field(h: Holder<Disk>) {
    h.item.save();
}

fn by_result(w: Wrap<Cloud>) {
    w.get().save();
}

fn main() {
    by_field(Holder { item: Disk });
    by_result(Wrap(Cloud));
}
