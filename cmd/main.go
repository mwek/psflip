package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/kkyr/fig"
	"github.com/mwek/psflip/pkg/figs"
	"github.com/mwek/psflip/pkg/healthcheck"
	"github.com/mwek/psflip/pkg/process"

	flag "github.com/spf13/pflag"
)

// Config describes the psflip configuration
type Config struct {
	// Cmd stores the child command to start
	Cmd []figs.TString `validate:"required"`
	// Env stores the extra environment to be passed to the child process
	Env []figs.TString
	// Pidfile (if not empty) stores the path with the PID of the active psflip
	Pidfile figs.TString
	// UpgradeTimeout specifies the maximum duration for the healthcheck
	UpgradeTimeout time.Duration `fig:"upgrade_timeout" default:"1m"`
	// ChildTimeout specifies the maximum duration for the Child to exit gracefully before being KILLed
	ChildTimeout time.Duration `fig:"child_timeout" default:"5s"`

	// Signals describe the control signals for psflip
	Signals struct {
		// Upgrade is the signal captured by psflip to initiate the upgrade process
		Upgrade figs.Signal `default:"SIGHUP"`
		// Terminate signal is sent to the Child when asked to shut down gracefully
		Terminate figs.Signal `default:"SIGTERM"`
	}

	// Healthcheck describes when to assume the child is healthy.
	Healthcheck healthcheck.Config
}

var (
	configFile     = flag.StringP("config", "c", "config.yml", "psflip configuration file")
	config         Config
	child          *process.Process
	ctx, ctxCancel = context.WithCancel(context.Background())
)

func exit(code int, format string, v ...any) {
	if code < 0 {
		code = 1
	}
	ctxCancel()
	if child != nil && child.Exited == nil {
		log.Printf("worker %s: sending %s", child, config.Signals.Terminate)
		child.Signal(config.Signals.Terminate.Syscall())
		select {
		case <-child.Done:
			// We are all good
		case <-time.After(config.ChildTimeout):
			log.Printf("worker %s: did not exit in %s, sending SIGKILL", child, config.ChildTimeout)
			child.Kill()
			<-child.Done
			// consider SIGKILL abnormal exit
			code = max(code, 1)
		}
	}
	log.Printf(format, v...)
	os.Exit(code)
}

func handleSignals(upg *tableflip.Upgrader) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	for s := range sig {
		switch {
		case s == config.Signals.Upgrade.Syscall():
			upg.Upgrade()
		default:
			if child != nil {
				child.Signal(s)
			}
		}
	}
}

func main() {
	// clean exit on panics
	defer func() {
		if r := recover(); r != nil {
			exit(1, "panic: %v", r)
		}
	}()

	flag.Parse()
	err := fig.Load(&config, fig.File(*configFile))
	if err != nil {
		exit(1, "failed to parse configuration: %v", err)
	}

	hc, err := healthcheck.New(config.Healthcheck)
	if err != nil {
		exit(1, "failed to configure healthcheck: %v", err)
	}

	// Support zero-downtime upgrades
	upg, _ := tableflip.New(tableflip.Options{
		PIDFile:        config.Pidfile.String(),
		UpgradeTimeout: config.UpgradeTimeout + 5*time.Second, // 5s buffer should prevent tableflip kills
	})
	defer upg.Stop()

	// Start child process
	child, err = process.Start(figs.Stringify(config.Cmd), figs.Stringify(config.Env))
	if err != nil {
		exit(1, "failed to start worker: %v", err)
	}

	// Proxy signals
	go handleSignals(upg)

	// Ensure we are healthy
	select {
	case <-time.After(config.UpgradeTimeout):
		exit(1, "worker %s: unhealthy, did not settle after %s", child, config.UpgradeTimeout)
	case ps := <-child.Done:
		ec := process.ExitCode(ps)
		exit(max(ec, 1), "worker %s: unhealthy, process exited with %d", child, ec)
	case err = <-hc.Healthy(ctx):
		if err != nil {
			exit(1, "worker %s: unhealthy, healthchek failed: %v", child, err)
		}
	}

	// We are all good, signal upgrade completed
	if err := upg.Ready(); err != nil {
		exit(1, "worker %s: unhealthy, failed to signal ready: %v", child, err)
	}
	log.Printf("worker %s healthy", child)

	// exit on upgrade or on child exit
	select {
	// Upgrade cleanup: terminate the child process
	case <-upg.Exit():
		exit(0, "worker %s: shutdown, upgrade completed", child)
	// Child exit: cleanup and proxy error code
	case ps := <-child.Done:
		os.Remove(config.Pidfile.String())
		ec := process.ExitCode(ps)
		exit(ec, "worker %s exited with %d, exiting", child, ec)
	}
}
