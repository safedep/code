package main

import "os"

type any func()

func clear() {
	os.Remove("x")
}

func rm() {
	os.Getenv("x")
}
