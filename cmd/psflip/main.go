package main

import (
	"context"
	"encoding/gob"
	logger "log"
	"os"
	"os/signal"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/kkyr/fig"
	"github.com/mwek/psflip/pkg/figs"
	"github.com/mwek/psflip/pkg/healthcheck"

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
	fConfig = flag.StringP("config", "c", "config.yml", "psflip configuration file")
	config  Config
)

func log(format string, v ...any) {
	if config.Quiet {
		return
	}
	logger.Printf(format, v...)
}

func upgrade(ctx context.Context, upg *tableflip.Upgrader) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, config.Upgrade.Signal.Syscall())
	for {
		select {
		case <-sig:
			if err := upg.Upgrade(); err != nil {
				log("error upgrading: %v", err)
				// TODO(mwek): systemd-notify READY=1 -- systemd doesn't seem to have
			}
		case <-ctx.Done():
			return
		}
	}
}

func pidForwarder(upg *tableflip.Upgrader) (r *os.File, w *os.File, err error) {
	// Clean returned pipes on error
	defer func() {
		if err != nil {
			r.Close()
			r = nil
			w.Close()
			w = nil
		}
	}()

	// New pipe -- reader for myslef, writer for the child
	r, childW, err := os.Pipe()
	if err != nil {
		return
	}
	defer childW.Close() // dup'ed by upgrader
	// Get inherited writer from upgrader
	w, err = upg.File("psflip-pidforwarder")
	if err != nil {
		return
	}
	// Set upgrade writer to propagate to the next child
	err = upg.AddFile("psflip-pidforwarder", childW)
	return
}

func main() {
	flag.Parse()
	err := fig.Load(&config, fig.File(*fConfig))
	if err != nil {
		logger.Fatalf("invalid psflip configuration: %v", err)
	}

	// Support zero-downtime upgrades
	buffer := 5 * time.Second // extra buffer to prevent kills from tableflip
	upg, err := tableflip.New(tableflip.Options{
		PIDFile:        config.Pidfile.String(),
		UpgradeTimeout: config.Upgrade.Timeout + config.Shutdown.Timeout + buffer,
	})
	if err != nil {
		logger.Fatalf("invalid tableflip configuration: %v", err)
	}
	defer upg.Stop()

	sv, err := newSupervisor(&config)
	if err != nil {
		logger.Fatalf("faild to create supverisor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = sv.Start(ctx)
	if err != nil {
		logger.Fatalf("failed to start child process: %v", err)
	}

	// On return, terminate the supervisor and proxy exit code
	defer func() {
		if sv != nil {
			cancel()
			<-sv.Exit()
			os.Exit(sv.ExitCode())
		}
	}()

	// Handle upgrade signals
	go upgrade(ctx, upg)

	// Setup PID-forwarder pipe
	pidpipeR, pidpipeW, err := pidForwarder(upg)
	if err != nil {
		pidpipeR, pidpipeW = nil, nil
	}
	defer pidpipeR.Close()
	defer pidpipeW.Close()

	select {
	case <-sv.Exit(): // supervisor never got ready
		return
	case <-sv.Ready(): // we are healthy
	}

	// Signal we are ready
	err = gob.NewEncoder(pidpipeW).Encode(os.Getpid())
	if pidpipeW != nil && err != nil {
		log("failed to write child pid: %v", err)
	}
	err = upg.Ready()
	if err != nil {
		log("failed to signal ready: %v", err)
		return
	}

	// exit on upgrade or on child exit
	select {
	// Upgrade cleanup: notify about PID change
	case <-upg.Exit():
		var childPid int
		if gob.NewDecoder(pidpipeR).Decode(&childPid) == nil {
			// # TODO(mwek): systemd-notify --pid &childPid
		}
	// Child exit: cleanup and proxy error code
	case <-sv.Exit():
		os.Remove(config.Pidfile.String())
	}
}
