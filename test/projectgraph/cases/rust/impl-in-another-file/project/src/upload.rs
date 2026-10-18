use std::net::TcpStream;

use crate::report::Report;

impl Report {
    pub fn upload(&self) {
        TcpStream::connect("example.com:80").unwrap();
    }
}
