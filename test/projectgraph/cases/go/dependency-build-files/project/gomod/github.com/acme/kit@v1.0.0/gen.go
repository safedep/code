//go:build ignore

package main

import "os"

func main() {
	os.WriteFile("tables.go", nil, 0o644)
}
