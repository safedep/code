package main

import (
	"os/exec"

	_ "example.com/shop/lib"
)

func main() {
	exec.Command("ls")
}
