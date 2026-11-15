package fast

import "os/exec"

// Sum is in sum_amd64.s, which calls fallback.
func Sum(xs []int) int

func fallback() { exec.LookPath("ls") }
