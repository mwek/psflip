//go:build linux

package process

import "syscall"

func SysAttr() *syscall.SysProcAttr {
	// Pdeathsig instructs the kernel to SIGKILL the child if psflip dies.
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}
