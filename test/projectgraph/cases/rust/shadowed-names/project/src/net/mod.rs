pub fn send() {
    std::net::TcpStream::connect("net:1").unwrap();
}
