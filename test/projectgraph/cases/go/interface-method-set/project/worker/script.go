package worker

import "os/exec"

type Script struct{}

func (Script) Run(name string) error {
	return exec.Command("sh", name).Run()
}
