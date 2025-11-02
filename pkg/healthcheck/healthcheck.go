package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/kkyr/fig"
)

// Config specifies available healthchecks. Only one must be configured
type Config struct {
	Alive   *Alive
	Command *Command
	Docker  *Docker
}

// Healtcheck determines if the cmd is healthy
type Healthcheck interface {
	Healthy(ctx context.Context) <-chan error
}

// New returns the configured healthcheck. It errors when more than one config is present.
func New(c Config) (Healthcheck, error) {
	val := reflect.ValueOf(c)
	var name string
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if name != "" {
				return nil, errors.New("multiple healthchecks configured")
			}
			name = val.Type().Field(i).Name
		}
	}
	if name == "" {
		return &def, nil
	}
	hc, ok := val.FieldByName(name).Interface().(Healthcheck)
	if !ok {
		return nil, fmt.Errorf("healthcheck %s does not implement Healthcheck interface", name)
	}
	return hc, nil
}

// def is the default healthcheck
var def Alive

func init() {
	fig.Load(&def, fig.IgnoreFile())
}
