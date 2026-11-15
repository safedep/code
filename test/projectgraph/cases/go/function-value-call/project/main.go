package main

import (
	"os"
	"os/exec"

	"example.com/app/backoff"
)

type handler interface {
	Handle()
}

type local struct{}

func (local) Handle() {
	exec.LookPath("ls")
}

func run() {
	os.Getwd()
}

func unused() {
	os.Remove("x")
}

func main() {
	backoff.Linear(2)(3)
	var h handler = local{}
	fns := []func(){h.Handle}
	fns[0]()
	ch := make(chan func(), 1)
	ch <- run
	more := make([]func(), 1)
	more[0] = <-ch
	more[0]()
}
