package main

import (
	"os"

	_ "example.com/app/handlers"
)

type server struct {
	Handle func(string) error
}

func (s *server) serve() {
	s.Handle("/")
}

func main() {
	s := &server{Handle: os.Chdir}
	s.serve()
}
