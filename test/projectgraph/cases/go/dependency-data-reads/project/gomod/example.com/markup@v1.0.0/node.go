package markup

import (
	"errors"
	"os/exec"
)

type Attribute struct {
	Key, Val string
}

type Node struct {
	Data string
	Attr []Attribute
}

const KindText = 1

var ErrNoRows = errors.New("no rows")

var Handler = func(s string) {
	exec.Command("handler", s)
}

var Run = run

func run(s string) {
	exec.Command("run", s)
}

func Print(v any) {
	exec.Command("print")
}

func Render(n *Node) {
	Print(n.Data)
	Print(ErrNoRows)
	Print(KindText)
	Handler("render")
	Run("render")
}
