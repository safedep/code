package a

import "github.com/acme/b"

func F() {
	b.Get(1)()
}
