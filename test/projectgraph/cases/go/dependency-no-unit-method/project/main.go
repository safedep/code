package main

import (
	"os"

	"example.com/legacy"
)

type closer interface {
	Close() error
}

var conns = []closer{&legacy.Conn{}}

func run() {
	conns[0].Close()
}

func idle() {}

func purge() {
	os.RemoveAll("data")
}

func main() {
	run()
}
