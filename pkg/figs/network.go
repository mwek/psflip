package figs

import (
	"fmt"
	"strings"

	"github.com/kkyr/fig"
)

type NetworkAddr struct {
	Network string
	Address string
}

// Network implements fig.StringUnmarshaler
func (p *NetworkAddr) UnmarshalString(str string) error {
	// Support template substitution
	var tmpl TString
	err := tmpl.UnmarshalString(str)
	if err != nil {
		return err
	}
	str = tmpl.String()

	// Parse network and address
	var network, address string
	parts := strings.SplitN(str, "://", 2)
	if len(parts) == 2 {
		network = strings.ToLower(parts[0])
		address = parts[1]
	} else {
		network = "tcp"
		address = str
	}

	switch network {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		// Values supported by net.Listen: https://pkg.go.dev/net#Listen
	default:
		return fmt.Errorf("invalid network: %s", str)
	}
	*p = NetworkAddr{network, address}
	return nil
}

// Network implements fmt.Stringer
func (s NetworkAddr) String() string {
	return fmt.Sprintf("%s://%s", s.Network, s.Address)
}

// Compile-time check for interface implementation
var _ fmt.Stringer = NetworkAddr{Network: "tcp", Address: "127.0.0.1:8080"}
var _ fig.StringUnmarshaler = (*NetworkAddr)(nil)
