use std::fmt;
use std::ops::{Add, Deref};

pub struct Pretty {}

impl fmt::Display for Pretty {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        std::process::Command::new("display").spawn().unwrap();
        write!(f, "pretty")
    }
}

pub struct Guard {}

impl Drop for Guard {
    fn drop(&mut self) {
        std::fs::remove_file("lock").unwrap();
    }
}

pub struct Money(pub i32);

impl Add for Money {
    type Output = Money;
    fn add(self, other: Money) -> Money {
        std::net::TcpStream::connect("add:1").unwrap();
        Money(self.0 + other.0)
    }
}

pub struct Counter {
    n: i32,
}

impl Counter {
    pub fn new() -> Self {
        Counter { n: 0 }
    }
}

impl Iterator for Counter {
    type Item = i32;
    fn next(&mut self) -> Option<i32> {
        std::thread::sleep(std::time::Duration::from_secs(1));
        None
    }
}

pub struct Inner {}

impl Inner {
    pub fn inner_call(&self) {
        std::fs::remove_dir("inner").unwrap();
    }
}

pub struct Wrapper {
    pub inner: Inner,
}

impl Deref for Wrapper {
    type Target = Inner;
    fn deref(&self) -> &Inner {
        &self.inner
    }
}

pub struct MyError {}

impl From<std::io::Error> for MyError {
    fn from(_: std::io::Error) -> Self {
        std::fs::write("err", "x").unwrap();
        MyError {}
    }
}
