package main

import (
	"crypto/tls"
	"fmt"
	"net"
)

func main() {
	fmt.Println("serve")
}

func unused(c net.Conn) {
	tls.Client(c, nil)
}
