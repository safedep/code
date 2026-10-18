package app

import (
	"net/http"

	"example.com/app/store"
)

type Reader interface {
	Read(p []byte) (int, error)
}

type ReadCloser interface {
	Reader
	Close() error
}

type Server struct {
	Reader
	*store.Store
	name, addr string
	db         *store.DB
	cache      map[string]int
}

type Box[T any] struct{ v T }

func (b *Box[T]) Get() T { return b.v }

func NewServer(addr string, opts ...string) (*Server, error) {
	return &Server{addr: addr}, nil
}

func (s *Server) Run(h http.Handler, n int) {
	db := store.Open("x")
	var r Reader = s
	c := &Server{}
	d, err := NewServer("y")
	e := new(store.DB)
	r.Read(nil)
	c.Run(h, n)
	d.Run(nil, 0)
	e.Close()
	db.Query("q")
	f := s.Close
	http.HandleFunc("/", s.serve)
	go func() { f() }()
	_ = err
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {}

func (s *Server) Close() error { return nil }
