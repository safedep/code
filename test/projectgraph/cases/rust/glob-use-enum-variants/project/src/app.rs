use crate::kinds::Fast;
use crate::kinds::Kind;

pub fn start(k: Kind) {
    let _m = Fast;
    crate::kinds::run(k);
}
