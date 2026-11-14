package main

import (
	"os"

	"example.com/legacy"
)

type store struct{}

func (store) Close() error {
	legacy.Flush()
	return nil
}

func purge() {
	os.RemoveAll("data")
}

func main() {
	os.Getenv("HOME")
}
