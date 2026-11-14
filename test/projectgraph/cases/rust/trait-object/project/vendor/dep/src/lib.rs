pub trait Write {
    fn write(&self);
}

pub struct Html {}

impl Write for Html {
    fn write(&self) {}
}
