package healthcheck

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/mwek/psflip/pkg/figs"
	"github.com/mwek/psflip/pkg/process"
)

// Command healthcheck executes the command specified, and assumes the child process healthy
// when it exits successfully.
type Command struct {
	Cmd      []figs.TString `validate:"required"`
	After    time.Duration  `default:"0s"`
	Interval time.Duration  `default:"1s"`
}

// compile-time check for interface implementation
var _ Healthcheck = &Command{}

func (c *Command) Healthy(ctx context.Context) <-chan error {
	result := make(chan error, 1)
	go c.check(ctx, result)
	return result
}

func (c *Command) check(ctx context.Context, result chan error) {
	// Wait for "After"
	select {
	case <-ctx.Done():
		result <- errors.New("context cancelled")
		return
	case <-time.After(c.After):
	}

	r := newRunner()
	cmd := figs.Stringify(c.Cmd)
	for {
		select {
		case <-time.Tick(c.Interval):
			go r.run(ctx, cmd)
		case err := <-r.result:
			if err == nil {
				result <- nil
				return
			}
		case <-ctx.Done():
			result <- errors.New("context cancelled")
			return
		}
	}
}

type runner struct {
	mutex  sync.Mutex
	result chan error
}

func newRunner() *runner {
	return &runner{
		result: make(chan error, 1),
	}
}

func (r *runner) run(ctx context.Context, command []string) {
	// allow one runner at a time
	if !r.mutex.TryLock() {
		return
	}
	defer r.mutex.Unlock()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.SysProcAttr = process.SysAttr()
	r.result <- cmd.Run()
}
