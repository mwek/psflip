package main

import (
	"context"
	logger "log"
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
	// Pidfile (if not empty) describes the path with the PID of the active psflip
	Pidfile figs.TString
	// Quiet suppresses any log output from printf
	Quiet bool

	// Upgrade controls the psflip upgrade process.
	Upgrade struct {
		// Signal initiating the upgrade process
		Signal figs.Signal `default:"SIGHUP"`
		// Timeout after the child is considered "unhealthy"
		Timeout time.Duration `default:"1m"`
	}

	// Shutdown controls the child's graceful shutdown.
	Shutdown struct {
		// Signal sent to the child asking for graceful shutdown.
		Signal figs.Signal `default:"SIGTERM"`
		// Timeout after the child receives the SIGKILL.
		Timeout time.Duration `default:"10s"`
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

func log(format string, v ...any) {
	if config.Quiet {
		return
	}
	logger.Printf(format, v...)
}

func exit(code int, format string, v ...any) {
	if code < 0 {
		code = 1
	}
	ctxCancel()
	if child != nil && child.Exited == nil {
		log("worker %s: sending %s", child, config.Shutdown.Signal)
		child.Signal(config.Shutdown.Signal.Syscall())
		select {
		case <-child.Done:
			// We are all good
		case <-time.After(config.Shutdown.Timeout):
			log("worker %s: did not exit in %s, sending SIGKILL", child, config.Shutdown.Timeout)
			child.Kill()
			<-child.Done
			// consider SIGKILL abnormal exit
			code = max(code, 1)
		}
	}
	log(format, v...)
	os.Exit(code)
}

func handleSignals(upg *tableflip.Upgrader) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	for s := range sig {
		switch {
		case s == config.Upgrade.Signal.Syscall():
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
		UpgradeTimeout: config.Upgrade.Timeout + 5*time.Second, // 5s buffer should prevent tableflip kills
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
	case <-time.After(config.Upgrade.Timeout):
		exit(1, "worker %s: unhealthy, did not settle after %s", child, config.Upgrade.Timeout)
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
	log("worker %s healthy", child)

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
