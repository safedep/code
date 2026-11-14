package main

import (
	"os"

	"github.com/acme/kit"
)

func main() {
	os.Getenv("HOME")
	kit.Run()
}
