package main

import (
	"os"
	"os/exec"
)

func walk(yield func(string) bool) {
	os.Remove("x")
	yield("a")
}

type tree struct{}

func (tree) All(yield func(int) bool) {
	exec.LookPath("ls")
	yield(1)
}

func unused(yield func(string) bool) {
	os.Getwd()
}

func main() {
	for range walk {
	}
	var t tree
	for x := range t.All {
		_ = x
	}
	for _, s := range []string{"a"} {
		_ = s
	}
}
