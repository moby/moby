package bridge

import (
	"fmt"
	"io/ioutil"
)

const (
	ipv4ForwardConf     = "/proc/sys/net/ipv4/ip_forward"
	ipv4ForwardConfPerm = 0644
)

func setupIPForwarding(config *configuration) error {
	// Sanity Check
	if config.EnableIPForwarding == false {
		return &ErrIPFwdCfg{}
	}

	// Enable IPv4 forwarding
	if err := ioutil.WriteFile(ipv4ForwardConf, []byte{'1', '\n'}, ipv4ForwardConfPerm); err != nil {
		return fmt.Errorf("Setup IP forwarding failed: %v", err)
	}

	return nil
}
