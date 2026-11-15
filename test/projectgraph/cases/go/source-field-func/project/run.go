package main

type Config struct {
	Remove func(string) error
}

type Svc struct {
	cfg Config
}

func (s *Svc) run() {
	s.cfg.Remove("x")
}
