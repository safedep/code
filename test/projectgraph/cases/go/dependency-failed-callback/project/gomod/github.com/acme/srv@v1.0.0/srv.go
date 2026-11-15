package srv

type Handler interface {
	Handle()
}

func Serve(h Handler) {
	serve(h)
}
