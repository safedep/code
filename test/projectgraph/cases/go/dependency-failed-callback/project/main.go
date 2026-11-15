package main

import (
	"os/exec"

	"github.com/acme/srv"
)

type mine struct{}

func (mine) Handle() {
	exec.LookPath("ls")
}

func main() {
	srv.Serve(mine{})
}
