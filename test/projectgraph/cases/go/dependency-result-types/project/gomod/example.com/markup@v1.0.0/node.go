package markup

import (
	"io"
	"os"
)

type Node struct{}

func Parse(r io.Reader) (*Node, error) {
	return &Node{}, nil
}

func (n *Node) Render(w io.Writer) {
	os.Remove("render")
}
