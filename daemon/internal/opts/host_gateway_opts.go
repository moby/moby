package opts

import (
	"errors"
	"net/netip"
)

// ValidateHostGatewayIPs makes sure the addresses are valid, and there's at-most one IPv4 and one IPv6 address.
func ValidateHostGatewayIPs(hostGatewayIPs []netip.Addr) error {
	var have4, have6 bool
	for _, ip := range hostGatewayIPs {
		if ip.Is4() {
			if have4 {
				return errors.New("only one IPv4 host gateway IP address can be specified")
			}
			have4 = true
		} else {
			if have6 {
				return errors.New("only one IPv6 host gateway IP address can be specified")
			}
			have6 = true
		}
	}
	return nil
}
