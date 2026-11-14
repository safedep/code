package main

import (
	"os"

	_ "example.com/app/plugins"
	"example.com/app/registry"
)

var fs = make([]func(string) error, 1)

func setup() {
	fs[0] = os.Chdir
}

func serve() {
	fs[0]("/")
}

func main() {
	setup()
	serve()
	registry.ByName()
	registry.First()
}
