package main

import (
	"os"

	"example.com/app/impl"
	"example.com/app/lib"
)

var hooks = []func(){os.Exit}

type options struct {
	Hook func()
}

var defaults = options{Hook: os.Getwd}

func run() {
	hooks[0]()
}

func runHook(o options) {
	o.Hook()
}

func idle() {}

func main() {
	run()
	runHook(defaults)
	lib.Serve(impl.T{})
}
