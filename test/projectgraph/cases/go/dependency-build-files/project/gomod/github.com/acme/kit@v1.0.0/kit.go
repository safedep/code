package kit

import "os/exec"

func Run(name string) {
	_ = exec.Command(name)
	_ = lookup(name)
}
