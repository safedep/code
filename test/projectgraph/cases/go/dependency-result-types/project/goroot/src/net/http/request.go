package http

import (
	"io"
	"os"
)

type Request struct{}

func NewRequest(method, url string, body io.Reader) (*Request, error) {
	return &Request{}, nil
}

func (r *Request) Write(w io.Writer) error {
	os.Getenv("write")
	return nil
}
