package main

import (
	"io"

	"golang.org/x/net/html"
)

func run(r io.Reader) {
	html.NewTokenizer(r).Next()
}
