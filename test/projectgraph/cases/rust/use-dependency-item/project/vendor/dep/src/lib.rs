pub mod sub;
mod hidden;

pub use hidden::reexported;

pub fn top() {
    sink();
}

pub fn sink() {}

pub fn renamed() {
    renamed_sink();
}

fn renamed_sink() {}

pub fn other() {
    other_sink();
}

fn other_sink() {}

pub fn inner() {
    inner_sink();
}

fn inner_sink() {}

pub fn unused() {
    unused_sink();
}

fn unused_sink() {}

pub struct Type;

impl Type {
    pub fn new() -> Type {
        Type
    }

    pub fn run(&self) {
        run_sink();
    }
}

fn run_sink() {}
