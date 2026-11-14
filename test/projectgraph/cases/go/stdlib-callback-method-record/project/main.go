package main

import (
	"net/http"
	"os"
)

type handler struct{}

func (handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	os.Remove("served")
}

type lister interface {
	ListenAndServe() error
}

type app struct {
	serve func(string, http.Handler) error
}

var srv = &http.Server{Handler: handler{}}

var servers = []*http.Server{{Handler: handler{}}}

var a = app{serve: http.ListenAndServe}

func viaVariable() {
	srv.ListenAndServe()
}

func viaIndex() {
	servers[0].ListenAndServe()
}

func viaRange(m map[string]*http.Server) {
	for _, s := range m {
		s.ListenAndServe()
	}
}

func viaInterface(l lister) {
	l.ListenAndServe()
}

func viaField() {
	a.serve(":80", handler{})
}

func viaMethodValue() {
	serve := srv.ListenAndServe
	serve()
}

func main() {}
