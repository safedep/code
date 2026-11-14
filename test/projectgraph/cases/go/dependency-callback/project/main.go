package main

import (
	"os"
	"os/exec"

	"github.com/acme/srv"
)

type mine struct{}

func (mine) Handle() {
	exec.LookPath("ls")
}

type conn struct{}

func (conn) Close() {
	os.Chdir("/")
}

func named() {
	os.Getwd()
}

func main() {
	srv.Run(named)
	srv.Each([]func(){func() {
		os.Getenv("X")
	}})
	srv.Serve(mine{})
	var c srv.Closer = conn{}
	srv.Bind(c.Close)
}
