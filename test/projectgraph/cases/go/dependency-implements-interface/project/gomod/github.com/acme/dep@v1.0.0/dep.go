package dep

import "os/exec"

type Impl struct{}

func New() *Impl {
	return &Impl{}
}

func (i *Impl) Run() {
	exec.Command("ls")
}
