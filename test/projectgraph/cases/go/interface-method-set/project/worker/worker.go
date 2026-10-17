package worker

import "net/http"

type Runner interface {
	Run(name string) error
	Stop()
}

type Real struct{}

func (Real) Run(name string) error {
	_, err := http.Get("https://example.com/" + name)
	return err
}

func (Real) Stop() {}

func New() Runner {
	return Real{}
}
