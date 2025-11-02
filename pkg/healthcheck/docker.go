package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/mwek/psflip/pkg/figs"
)

// Docker healthcheck waits until specified container is healthy.
type Docker struct {
	Container figs.TString  `validate:"required"`
	Socket    figs.TString  `default:"unix:///var/run/docker.sock"`
	Interval  time.Duration `default:"1s"`
}

// compile-time check for interface implementation
var _ Healthcheck = &Docker{}

func (d *Docker) Healthy(ctx context.Context) <-chan error {
	result := make(chan error, 1)
	go d.check(ctx, result)
	return result
}

func (d *Docker) check(ctx context.Context, result chan error) {
	c, err := client.New(client.WithHost(d.Socket.String()), client.WithAPIVersionNegotiation())
	if err != nil {
		result <- err
		return
	}
	defer c.Close()

	containerID := d.Container.String()
	for {
		select {
		case <-time.Tick(d.Interval):
			status, err := c.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
			if err != nil {
				continue
			}
			state := status.Container.State
			if state == nil {
				// TODO(mwek): should we fail here?
				continue
			}
			if state.Status == container.StateDead || state.Status == container.StateExited {
				result <- fmt.Errorf("container %s is %s", containerID, state.Status)
				return
			}
			if state.Health == nil {
				// TODO(mwek): should we fail here?
				continue
			}
			if state.Health.Status == container.Healthy {
				result <- nil
				return
			}
		case <-ctx.Done():
			result <- errors.New("context cancelled")
			return
		}
	}
}
