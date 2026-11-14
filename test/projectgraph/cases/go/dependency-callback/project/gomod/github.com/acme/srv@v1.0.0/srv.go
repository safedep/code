package srv

type Handler interface {
	Handle()
}

type Closer interface {
	Close()
}

func Run(f func()) {
	f()
}

func Each(fs []func()) {
	fs[0]()
}

func Serve(h Handler) {
	h.Handle()
}

func Bind(f func()) {
	f()
}
