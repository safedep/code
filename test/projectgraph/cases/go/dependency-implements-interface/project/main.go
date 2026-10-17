package main

import (
	"example.com/app/iface"
	"github.com/acme/dep"
)

func main() {
	var r iface.Runner = dep.New()
	r.Run()
}
