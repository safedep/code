package main

import (
	"os"
	"os/exec"
)

type local struct{}

func (local) run() { os.Getwd() }

type remote struct{}

func (remote) run() { exec.LookPath("ls") }

func (remote) stop() { os.Remove("x") }

func main() {
	x := local{}
	x.run()
	{
		x := remote{}
		x.run()
		f := x.stop
		f()
	}
}
