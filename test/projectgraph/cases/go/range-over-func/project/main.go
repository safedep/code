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

type source struct {
	each func(func(string) bool)
}

func consume(seq func(func(string) bool)) {
	for range seq {
	}
}

func drain(s source) {
	for x := range s.each {
		_ = x
	}
}

type element struct{}

func (element) Attr() string {
	os.Chdir("/")
	return ""
}

func attrs(ns []*node) {
	for _, n := range ns {
		for _, a := range n.Attr {
			_ = a
		}
	}
}

func typed(seq Seq) {
	for range seq {
	}
}

func viaLocal(h holder) {
	seq := h.Each
	for range seq {
	}
}
