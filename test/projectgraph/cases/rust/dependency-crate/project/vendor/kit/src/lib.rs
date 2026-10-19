mod store;

pub use store::Store;

pub trait Sink {
    fn flush(&self);
}
