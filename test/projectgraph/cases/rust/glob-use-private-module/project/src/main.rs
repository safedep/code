use dep::*;
use other::*;

mod inner;
mod user;

fn main() {
    hidden::run();
    user::call();
}
