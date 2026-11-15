package b

import (
	"os"
	"os/exec"
)

var table = map[int]func(){1: doA}

func init() {
	exec.Command("ls")
}

func doA() {
	os.Remove("x")
}

func doB() {
	os.Getwd()
}

func Get(i int) func() {
	return table[i]
}
