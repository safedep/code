package main

import (
	"os"
	"os/exec"
	"reflect"
)

type T struct{}

func (T) Hidden() {
	os.Remove("x")
}

type R struct{}

func (R) Method() {}

func other() {
	exec.Command("ls")
}

func main() {
	v := reflect.ValueOf(T{})
	v.MethodByName("Hidden").Call(nil)
	var r R
	r.Method()
}
