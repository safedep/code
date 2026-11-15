package lib

type Handler interface {
	Handle()
}

func Serve(h Handler) {
	h.Handle()
}
