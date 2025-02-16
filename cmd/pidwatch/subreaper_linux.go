//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func init() {
	if os.Getpid() == 1 || unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(1), 0, 0, 0) == nil {
		ticker = nil
		go func() {
			for {
				unix.Wait4(-1, nil, 0, nil)
				subreaper <- struct{}{}
			}
		}()
	}
}
