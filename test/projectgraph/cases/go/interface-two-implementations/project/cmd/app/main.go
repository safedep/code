package main

import "example.com/shop/sink"

func main() {
	run(sink.Pick())
}

func run(s sink.Sink) {
	s.Write("order")
}
