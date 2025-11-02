package figs

import (
	"fmt"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type Signal syscall.Signal

// Signal implements fig.StringUnmarshaler
func (s *Signal) UnmarshalString(str string) error {
	if str == "" {
		*s = Signal(0)
		return nil
	}

	sig := strings.ToUpper(str)
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + sig
	}
	*s = Signal(unix.SignalNum(sig))
	if *s == Signal(0) {
		return fmt.Errorf("invalid signal: %s", str)
	}
	return nil
}

func (s Signal) Syscall() syscall.Signal {
	return syscall.Signal(s)
}

func (s Signal) Valid() bool {
	return s.String() != ""
}

// Signal implements fmt.Stringer
func (s Signal) String() string {
	return unix.SignalName(s.Syscall())
}

var _ fmt.Stringer = Signal(syscall.SIGINT)
