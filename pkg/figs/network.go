package figs

import (
	"fmt"
	"strings"

	"github.com/kkyr/fig"
)

type Network string

// Network implements fig.StringUnmarshaler
func (p *Network) UnmarshalString(str string) error {
	str = strings.ToLower(str)
	switch str {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		// Values supported by net.Listen: https://pkg.go.dev/net#Listen
	default:
		return fmt.Errorf("invalid network: %s", str)
	}
	*p = Network(str)
	return nil
}

// Network implements fmt.Stringer
func (s Network) String() string {
	return string(s)
}

var _ fmt.Stringer = Network("tcp")
var _ fig.StringUnmarshaler = (*Network)(nil)
