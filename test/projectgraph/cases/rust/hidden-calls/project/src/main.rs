#[macro_use]
mod macros;
mod types;
mod ext;

use types::{Counter, Guard, Inner, Money, MyError, Pretty, Wrapper};
use ext::Shout;

fn helper() {
    std::process::abort();
}

fn worker(x: i32) -> i32 {
    std::env::set_var("W", "1");
    x
}

fn read() -> Result<(), MyError> {
    std::fs::read("x")?;
    Ok(())
}

fn main() {
    let f: fn() = helper;
    f();
    let v: Vec<i32> = vec![1].into_iter().map(worker).collect();
    println!("{}", Pretty {});
    let _g = Guard {};
    let _m = Money(1) + Money(2);
    for _ in Counter::new() {}
    let w = Wrapper { inner: Inner {} };
    w.inner_call();
    let s: String = String::new();
    s.shout();
    let _ = read();
    println!("{:?}", v);
    log_and_quit!();
}
