package main

import (
	"os"
	_ "unsafe"

	"github.com/acme/fast"
)

//go:linkname now runtime.nanotime
func now() int64

type closer struct{}

func (closer) Close() error { return os.Chdir("/") }

func unused() { os.Getwd() }

func viaAsm() { add(1, 2) }

func viaUnit() { fast.Sum(nil) }

func viaLinkname() { now() }

func main() {
	viaAsm()
	viaUnit()
	viaLinkname()
}

//go:linkname alias main.aliased
func alias()

func aliased() { os.Mkdir("d", 0o755) }

func viaAlias() { alias() }
