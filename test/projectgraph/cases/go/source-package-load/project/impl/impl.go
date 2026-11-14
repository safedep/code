package impl

import "os"

var table = map[string]func(){"rm": os.Remove}

type T struct{}

func (T) Handle() {
	table["rm"]()
}
