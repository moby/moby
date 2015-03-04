package bridge

import (
	"fmt"
	"io/ioutil"
)

const (
	IPV4_FORW_CONF_FILE = "/proc/sys/net/ipv4/ip_forward"
	PERM                = 0644
)

func SetupIPForwarding(i *Interface) error {
	// Sanity Check
	if i.Config.EnableIPForwarding == false {
		return fmt.Errorf("Unexpected request to enable IP Forwarding for: %v", *i)
	}

	// Enable IPv4 forwarding
	if err := ioutil.WriteFile(IPV4_FORW_CONF_FILE, []byte{'1', '\n'}, PERM); err != nil {
		return fmt.Errorf("Setup IP forwarding failed: %v", err)
	}

	return nil
}
