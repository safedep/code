package markup

import (
	"io"
	"os/exec"
)

type Tokenizer struct {
	r io.Reader
}

type Text struct{}

func NewTokenizer(r io.Reader) *Tokenizer {
	return &Tokenizer{r: r}
}

func (z *Tokenizer) Next() int {
	exec.Command("next")
	return 0
}

func (z *Tokenizer) Raw() *Text {
	return &Text{}
}

func (t *Text) Close() {
	exec.Command("close")
}
