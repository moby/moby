package opts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// HostGateway stores the IPv4 and IPv6 host-gateway endpoints, configured
// via a command line option or daemon.json. The addresses are stored as strings
// because they're only ever used as strings, and to avoid the need for a mergo
// Transformer to make the command-line/json config merge possible.
type HostGateway struct {
	V4, V6 string
}

// Set host gateway addresses, the value is expected to be at-most one IPv4
// and one IPv6 address, comma separated.
func (hg *HostGateway) Set(value string) error {
	for _, ipStr := range strings.Split(value, ",") {
		addr, err := netip.ParseAddr(ipStr)
		if err != nil {
			return fmt.Errorf("invalid IP address %q in option host-gateway-ip", ipStr)
		}
		if addr.Is4() {
			if hg.V4 != "" {
				return errors.New("at most one IPv4 address is allowed in option host-gateway-ip")
			}
			hg.V4 = addr.String()
		}
		if addr.Is6() {
			if hg.V6 != "" {
				return errors.New("at most one IPv6 address is allowed in option host-gateway-ip")
			}
			hg.V6 = addr.String()
		}
	}
	return nil
}

// UnmarshalJSON unmarshalls the JSON representation of host-gateway. The JSON
// representation is a comma separated string, not a JSON array of strings, for
// compatibility with the original option, which only allowed for a single address.
func (hg *HostGateway) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("invalid host-gateway-ip option: %w", err)
	}
	return hg.Set(s)
}

// String returns the string representation of the option.
func (hg *HostGateway) String() string {
	if hg.V4 != "" && hg.V6 != "" {
		return hg.V4 + "," + hg.V6
	}
	return hg.V4 + hg.V6
}

// Type returns the type of this option
func (hg *HostGateway) Type() string {
	return "host-gateway-option"
}
