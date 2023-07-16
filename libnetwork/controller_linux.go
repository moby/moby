package libnetwork

import (
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
)

// enabledIptablesVersions returns the iptables versions that are enabled
// for the controller.
func (c *Controller) enabledIptablesVersions() []iptables.IPVersion {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg == nil {
		return nil
	}
	// parse map cfg["bridge"]["generic"]["EnableIPTable"]
	cfgBridge := c.cfg.DriverConfig("bridge")
	cfgGeneric, ok := cfgBridge[netlabel.GenericData].(options.Generic)
	if !ok {
		return nil
	}

	var versions []iptables.IPVersion
	if enabled, ok := cfgGeneric["EnableIPTables"].(bool); enabled || !ok {
		// iptables is enabled unless user explicitly disabled it
		versions = append(versions, iptables.IPv4)
	}
	if enabled, _ := cfgGeneric["EnableIP6Tables"].(bool); enabled {
		versions = append(versions, iptables.IPv6)
	}
	return versions
}
