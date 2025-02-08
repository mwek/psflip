package healthcheck

import (
	"context"
	"errors"
	"time"
)

// Alive healthcheck assumes child process healthy after given timeout.
type Alive struct {
	Timeout time.Duration `default:"5s"`
}

// compile-time check for interface implementation
var _ Healthcheck = &Alive{}

func (a *Alive) Healthy(ctx context.Context) <-chan error {
	result := make(chan error, 1)
	go a.check(ctx, result)
	return result
}

func (a *Alive) check(ctx context.Context, result chan error) {
	select {
	case <-time.After(a.Timeout):
		result <- nil
	case <-ctx.Done():
		result <- errors.New("context cancelled")
	}
}
