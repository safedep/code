use crate::traits::*;
pub use crate::Mode::*;

pub enum Mode { Fast, Slow }
pub enum Kind { A, B }

impl Tr for Kind {}

pub fn run(k: Kind) {
    k.area();
}
