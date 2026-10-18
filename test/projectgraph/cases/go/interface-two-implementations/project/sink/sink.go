package sink

type Sink interface {
	Write(v string) error
}

func Pick() Sink {
	return File{}
}
