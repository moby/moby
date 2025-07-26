package libnetwork

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/options"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
)

// FirewallBackend returns the name of the firewall backend for "docker info".
func (c *Controller) FirewallBackend() *system.FirewallInfo {
	var info system.FirewallInfo
	info.Driver = "iptables"
	if nftables.Enabled() {
		info.Driver = "nftables"
	}
	if iptables.UsingFirewalld() {
		info.Driver += "+firewalld"
		if reloadedAt := iptables.FirewalldReloadedAt(); !reloadedAt.IsZero() {
			info.Info = [][2]string{{"ReloadedAt", reloadedAt.Format(time.RFC3339)}}
		}
	}
	return &info
}

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

// getDefaultOSLSandbox returns the controller's default [osl.Sandbox]. It
// creates the sandbox if it does not yet exist.
func (c *Controller) getDefaultOSLSandbox(key string) (*osl.Namespace, error) {
	var err error
	c.defOsSboxOnce.Do(func() {
		c.defOsSbox, err = osl.NewSandbox(key, false, false)
	})

	if err != nil {
		c.defOsSboxOnce = sync.Once{}
		return nil, fmt.Errorf("failed to create default sandbox: %v", err)
	}
	return c.defOsSbox, nil
}

// setupOSLSandbox sets the sandbox [osl.Sandbox], and applies operating-
// specific configuration.
//
// Depending on the Sandbox settings, it may either use the Controller's
// default sandbox, or configure a new one.
func (c *Controller) setupOSLSandbox(sb *Sandbox) error {
	if sb.config.useDefaultSandBox {
		defSB, err := c.getDefaultOSLSandbox(sb.Key())
		if err != nil {
			return err
		}
		sb.osSbox = defSB
	}

	if sb.osSbox == nil && !sb.config.useExternalKey {
		newSB, err := osl.NewSandbox(sb.Key(), !sb.config.useDefaultSandBox, false)
		if err != nil {
			return fmt.Errorf("failed to create new osl sandbox: %v", err)
		}
		sb.osSbox = newSB
	}

	if sb.osSbox != nil {
		// Apply operating specific knobs on the load balancer sandbox
		err := sb.osSbox.InvokeFunc(func() {
			sb.osSbox.ApplyOSTweaks(sb.oslTypes)
		})
		if err != nil {
			log.G(context.TODO()).Errorf("Failed to apply performance tuning sysctls to the sandbox: %v", err)
		}
		// Keep this just so performance is not changed
		sb.osSbox.ApplyOSTweaks(sb.oslTypes)
	}
	return nil
}
