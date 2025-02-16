package process

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"syscall"
)

var initialWD, _ = os.Getwd()
var initialEnv = os.Environ()

// Start is a wrapper on os.Process with Wait() signalled through channel
func Start(args []string, opts ...Option) (*Process, error) {
	executable, err := exec.LookPath(args[0])
	if err != nil {
		return nil, err
	}

	opt := options{}
	for _, o := range opts {
		o(&opt)
	}

	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	attr := &os.ProcAttr{
		Dir:   initialWD,
		Env:   slices.Concat(initialEnv, opt.env),
		Files: slices.Concat(files, opt.files),
		Sys:   SysAttr(),
	}

	p, err := os.StartProcess(executable, args, attr)
	if err != nil {
		return nil, err
	}

	done := make(chan *os.ProcessState, 1)
	c := &Process{p, executable, nil, done}
	// Always Wait() for the child process to finish
	go func() {
		st, err := p.Wait()
		c.Exited = st
		if err == nil {
			done <- st
		}
		close(done)
	}()
	return c, nil
}

type Process struct {
	*os.Process
	Name   string
	Exited *os.ProcessState
	Done   <-chan *os.ProcessState
}

func (p *Process) String() string {
	return fmt.Sprintf("%s[%d]", p.Name, p.Pid)
}

// ExitCode returns process exit code including signal termination
func ExitCode(ps *os.ProcessState) int {
	ws := ps.Sys().(syscall.WaitStatus)
	switch {
	case ws.Exited():
		return ws.ExitStatus()
	case ws.Signaled():
		return 128 + int(ws.Signal())
	default:
		return 1
	}
}
