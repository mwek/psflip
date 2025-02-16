package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/mwek/psflip/pkg/process"
	flag "github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

var (
	pidfile = flag.String("pidfile", "", "path to the pidfile")
	quiet   = flag.BoolP("quiet", "q", false, "suppress any output originating from pidwatch")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s [OPTIONS...] -- CHILD ...\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Start CHILD and wait until the process indicated by pidfile exits.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Available options:")
		flag.PrintDefaults()
	}
}

func getWatchedPid() int {
	if *pidfile == "" {
		return 0
	}
	pid, err := os.ReadFile(*pidfile)
	if err != nil {
		return -1
	}
	pidint, err := strconv.Atoi(string(pid))
	if err != nil {
		return -1
	}
	return pidint
}

func signalProxy() {
	sig := make(chan os.Signal, 100)
	signal.Notify(sig)
	for s := range sig {
		p := getWatchedPid()
		switch {
		case p <= 0:
			// ignore: no valid child
		case s == unix.SIGCHLD:
			// ignore: do not proxy child termination
		case s == unix.SIGPIPE:
			// ignore: do not proxy broken pipe writes
		case s == unix.SIGURG:
			// ignore: internal sigPreempt for Go runtime: https://github.com/golang/go/blob/go1.16.6/src/runtime/signal_unix.go#L41
		default:
			ps, _ := os.FindProcess(p)
			ps.Signal(s)
		}
	}
}

var (
	ticker    = time.Tick(100 * time.Millisecond)
	subreaper = make(chan struct{}, 10)
)

// wait signals on returned channel whenever a child pid should be checked for being alive
func wait() <-chan struct{} {
	events := make(chan struct{}, 10)
	go func() {
		for {
			select {
			case <-ticker:
			case <-subreaper:
			}
			events <- struct{}{}
		}
	}()
	return events
}

func main() {
	flag.Parse()

	if *quiet {
		log.Default().SetOutput(io.Discard)
	}
	if *pidfile == "" {
		log.Fatalf("pidwatch: --pidfile required")
	}

	go signalProxy()

	p, err := process.Start(flag.Args())
	if err != nil {
		log.Fatalf("pidwatch: failed to start child: %v", err)
	}

	<-p.Done

	waiter := wait()
	for range waiter {
		p := getWatchedPid()
		if p <= 0 {
			break
		}
		ps, _ := os.FindProcess(p)
		if ps.Signal(unix.Signal(0)) != nil {
			break
		}
	}
}
