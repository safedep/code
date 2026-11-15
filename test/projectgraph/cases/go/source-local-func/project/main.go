package main

import (
	"os"
	"os/exec"
)

type F func()

type hook struct {
	fn func()
}

func remove() { os.Remove("x") }

func lookup() { exec.LookPath("ls") }

func getwd() { os.Getwd() }

func getenv() { os.Getenv("HOME") }

func run(f func()) {
	f()
}

func each(fs []func()) {
	for _, g := range fs {
		g()
	}
}

func fire(hs []hook) {
	for _, h := range hs {
		h.fn()
	}
}

func call(h F) {
	var g F = h
	g()
}

func main() {
	run(remove)
	each([]func(){lookup})
	fire([]hook{{fn: getwd}})
	call(getenv)
}
