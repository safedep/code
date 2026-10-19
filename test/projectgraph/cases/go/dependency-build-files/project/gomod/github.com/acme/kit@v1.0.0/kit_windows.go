//go:build windows

package kit

import "syscall"

func Close() {
	syscall.CloseHandle(0)
}
