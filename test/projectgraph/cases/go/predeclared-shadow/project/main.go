package main

import "os"

var hook any = rm

func unused() {
	os.Getwd()
}

func main() {
	fs := []func(){clear}
	fs[0]()
	hook()
}
