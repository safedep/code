package main

import "os"

// add is in add_amd64.s, which calls slow.
func add(a, b int) int

func slow() { os.Remove("x") }
