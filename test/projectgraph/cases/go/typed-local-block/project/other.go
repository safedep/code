package main

import "os"

func helper() {
	os.Remove("x")
}

func cleanup() {
	os.Getwd()
}
