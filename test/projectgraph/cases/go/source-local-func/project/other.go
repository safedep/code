package main

import "net/http"

func serveLocal(m map[string]http.HandlerFunc) {
	var h http.HandlerFunc = m["a"]
	h(nil, nil)
}

func rangeAssign(fs []func()) {
	f := func() {}
	for _, f = range fs {
	}
	f()
}

func anonSteps(steps []struct{ act func() }) {
	for _, s := range steps {
		s.act()
	}
}

func deref(p *func()) {
	(*p)()
}

func keepRead(hs []hook) {
	for _, h := range hs {
		g := h.fn
		_ = g
	}
}
