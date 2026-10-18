use core_lib::{shout, Task};

struct Job {}

impl Task for Job {
    fn step(&self) {
        std::net::UdpSocket::bind("step:1").unwrap();
    }
}

fn main() {
    core_lib::transmit();
    core_lib::globbed();
    core_lib::a::b::deep();
    shout!(1);
    let j = Job {};
    j.run();
    Task::run(&j);
}
