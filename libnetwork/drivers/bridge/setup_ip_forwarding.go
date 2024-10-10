//go:build linux

package bridge

import (
	"fmt"
	"os"
)

const (
	ipv4ForwardConf     = "/proc/sys/net/ipv4/ip_forward"
	ipv4ForwardConfPerm = 0o644
)

func setupIPForwarding(enableIPTables bool, enableIP6Tables bool) error {
	// Get current IPv4 forward setup
	ipv4ForwardData, err := os.ReadFile(ipv4ForwardConf)
	if err != nil {
		return fmt.Errorf("Cannot read IP forwarding setup: %v", err)
	}

	// Enable IPv4 forwarding only if it is not already enabled
	if ipv4ForwardData[0] != '1' {
		if err := os.WriteFile(ipv4ForwardConf, []byte{'1', '\n'}, ipv4ForwardConfPerm); err != nil {
			return fmt.Errorf("Enabling IP forwarding failed: %v", err)
		}
	}
	return nil
}
