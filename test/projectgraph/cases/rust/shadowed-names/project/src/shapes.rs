pub struct Circle {}
pub struct Square {}

impl Circle {
    pub fn area(&self) -> f64 {
        std::env::var("circle").unwrap();
        1.0
    }
}

impl Square {
    pub fn area(&self) -> f64 {
        std::env::remove_var("square");
        1.0
    }

    pub fn area_of(side: i32) -> i32 {
        side * side
    }
}

pub trait Shape {
    fn area(&self) -> f64;
}

impl Shape for Square {
    fn area(&self) -> f64 {
        std::env::set_var("shape", "1");
        1.0
    }
}
