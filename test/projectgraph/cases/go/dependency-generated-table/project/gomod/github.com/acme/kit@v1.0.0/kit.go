package kit

import "os"

func Dispatch(name string) { handlers[name]() }

func remove() { os.Remove("x") }
