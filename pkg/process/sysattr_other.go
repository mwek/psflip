//go:build !linux

package process

import "syscall"

func SysAttr() *syscall.SysProcAttr {
	// Pdeathsig only supported on Linux
	return nil
}
