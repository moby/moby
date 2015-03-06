package bridge

import (
	"fmt"
	"io/ioutil"
)

const (
	ipv4ForwardConf     = "/proc/sys/net/ipv4/ip_forward"
	ipv4ForwardConfPerm = 0644
)

func setupIPForwarding(i *bridgeInterface) error {
	// Sanity Check
	if i.Config.EnableIPForwarding == false {
		return fmt.Errorf("Unexpected request to enable IP Forwarding for: %v", *i)
	}

	// Enable IPv4 forwarding
	if err := ioutil.WriteFile(ipv4ForwardConf, []byte{'1', '\n'}, ipv4ForwardConfPerm); err != nil {
		return fmt.Errorf("Setup IP forwarding failed: %v", err)
	}

	return nil
}
