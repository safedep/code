package main

import (
	"example.com/legacy"
	"github.com/acme/kit"
)

func main() {
	kit.Run()
	legacy.Start()
}
