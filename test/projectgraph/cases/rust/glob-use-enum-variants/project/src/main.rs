mod app;
mod kinds;
mod traits;
pub use kinds::*;
pub use kinds::Kind::*;

fn main() {
    app::start(Kind::A);
}
