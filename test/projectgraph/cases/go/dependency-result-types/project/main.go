package main

import (
	"net/http"

	"example.com/markup"
)

func main() {
	tokens()
	tree()
	send()
}

func tokens() {
	z := markup.NewTokenizer(nil)
	z.Next()
	markup.NewTokenizer(nil).Raw().Close()
}

func tree() {
	n, err := markup.Parse(nil)
	if err != nil {
		return
	}
	n.Render(nil)
}

func send() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Write(nil)
}
