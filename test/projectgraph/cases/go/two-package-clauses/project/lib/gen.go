//go:build ignore

package main

import "os"

func main() {
	os.WriteFile("table.go", nil, 0o600)
}

func parse() {}
