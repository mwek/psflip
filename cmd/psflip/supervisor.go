package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/mwek/psflip/pkg/figs"
	"github.com/mwek/psflip/pkg/healthcheck"
	"github.com/mwek/psflip/pkg/process"
	"golang.org/x/sys/unix"
)

type supervisor struct {
	*Config

	hc    healthcheck.Healthcheck
	ready chan struct{}
	exit  chan struct{}
	ec    int
}

func newSupervisor(c *Config) (*supervisor, error) {
	hc, err := healthcheck.New(c.Healthcheck)
	if err != nil {
		return nil, err
	}
	sv := &supervisor{
		Config: c,
		hc:     hc,
		ready:  make(chan struct{}),
		exit:   make(chan struct{}),
		ec:     -1,
	}
	return sv, nil
}

func (sv *supervisor) Ready() <-chan struct{} {
	return sv.ready
}

func (sv *supervisor) Exit() <-chan struct{} {
	return sv.exit
}

func (sv *supervisor) ExitCode() int {
	return sv.ec
}

func (sv *supervisor) cleanup(child *process.Process) {
	if child == nil || child.Exited != nil {
		return
	}

	log("worker %s: sending %s", child, sv.Shutdown.Signal)
	child.Signal(sv.Shutdown.Signal.Syscall())
	select {
	case ps := <-child.Done:
		sv.ec = process.ExitCode(ps)
	case <-time.After(sv.Shutdown.Timeout):
		log("worker %s: did not exit in %s, sending SIGKILL", child, sv.Shutdown.Timeout)
		child.Kill()
		<-child.Done
		// consider SIGKILL abnormal exit
		sv.ec = 1
	}
}

func (sv *supervisor) signal(ctx context.Context, child *process.Process) {
	sig := make(chan os.Signal, 10)
	signal.Notify(sig)
	for {
		select {
		case s := <-sig:
			switch {
			case child == nil:
				// ignore: no child to proxy
			case s == sv.Upgrade.Signal.Syscall():
				// ignore: skip upgrade signal
			case s == unix.SIGCHLD:
				// ignore: do not proxy child termination
			case s == unix.SIGPIPE:
				// ignore: do not proxy broken pipe writes
			case s == unix.SIGURG:
				// ignore: internal sigPreempt for Go runtime: https://github.com/golang/go/blob/go1.16.6/src/runtime/signal_unix.go#L41
			default:
				child.Signal(s)
			}
		case <-ctx.Done():
			signal.Stop(sig)
			return
		}
	}
}

func (sv *supervisor) Start(ctx context.Context) error {
	// Start child process
	env := figs.Stringify(sv.Env)
	child, err := process.Start(
		figs.Stringify(sv.Cmd),
		process.Env(env...),
		process.Dir(sv.WorkDir.String()),
	)
	if err != nil {
		return err
	}

	// Proxy signals
	go sv.signal(ctx, child)
	// Supervise execution
	go sv.supervise(ctx, child)
	return nil
}

func (sv *supervisor) supervise(ctx context.Context, child *process.Process) (ec int) {
	// Clean child process on exit
	defer close(sv.exit)
	defer sv.cleanup(child)
	defer func() { sv.ec = ec }()

	// Ensure we are healthy
	select {
	case <-time.After(sv.Upgrade.Timeout):
		defer log("worker %s: unhealthy, did not settle after %s", child, sv.Upgrade.Timeout)
		return 1
	case ps := <-child.Done:
		ec := process.ExitCode(ps)
		log("worker %s: unhealthy, process exited with %d", child, ec)
		return max(ec, 1)
	case err := <-sv.hc.Healthy(ctx):
		if err != nil {
			defer log("worker %s: unhealthy, healthchek failed: %v", child, err)
			return 1
		}
	}

	// Signal we are ready
	close(sv.ready)
	log("worker %s healthy", child)

	// exit on cancellation or on child exit
	select {
	// Cancellation: terminate the child process
	case <-ctx.Done():
		defer log("worker %s: shutdown, upgrade completed", child)
		return 0
	// Child exit: cleanup and proxy error code
	case ps := <-child.Done:
		ec = process.ExitCode(ps)
		log("worker %s exited with %d, exiting", child, ec)
		return ec
	}
}
