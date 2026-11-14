pub trait Write {
    fn write(&self) {}
}

pub struct Html {}

impl Write for Html {
    fn write(&self) {}
}

pub struct Plain {}

impl Plain {
    pub fn write(&self) {}
}

pub fn plain() -> Plain {
    Plain {}
}
