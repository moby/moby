//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/firewaller"
)

const (
	ipv4ForwardConf        = "/proc/sys/net/ipv4/ip_forward"
	ipv6ForwardConfDefault = "/proc/sys/net/ipv6/conf/default/forwarding"
	ipv6ForwardConfAll     = "/proc/sys/net/ipv6/conf/all/forwarding"
)

func setupIPv4Forwarding(fw firewaller.Firewaller, wantFilterForwardDrop bool) (retErr error) {
	changed, err := configureIPForwarding(ipv4ForwardConf, '1')
	if err != nil {
		return err
	}
	if changed {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv4ForwardConf, '0'); err != nil {
					log.G(context.TODO()).WithError(err).Error("Cannot disable IPv4 forwarding")
				}
			}
		}()
	}

	// When enabling ip_forward set the default policy on forward chain to drop.
	if changed && wantFilterForwardDrop {
		if err := fw.FilterForwardDrop(context.TODO(), firewaller.IPv4); err != nil {
			return err
		}
	}
	return nil
}

func setupIPv6Forwarding(fw firewaller.Firewaller, wantFilterForwardDrop bool) (retErr error) {
	// Set IPv6 default.forwarding, if needed.
	// FIXME(robmry) - is it necessary to set this, setting "all" (below) does the job?
	changedDef, err := configureIPForwarding(ipv6ForwardConfDefault, '1')
	if err != nil {
		return err
	}
	if changedDef {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv6ForwardConfDefault, '0'); err != nil {
					log.G(context.TODO()).WithError(err).Error("Cannot disable IPv6 default.forwarding")
				}
			}
		}()
	}

	// Set IPv6 all.forwarding, if needed.
	changedAll, err := configureIPForwarding(ipv6ForwardConfAll, '1')
	if err != nil {
		return err
	}
	if changedAll {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv6ForwardConfAll, '0'); err != nil {
					log.G(context.TODO()).WithError(err).Error("Cannot disable IPv6 all.forwarding")
				}
			}
		}()
	}

	if (changedAll || changedDef) && wantFilterForwardDrop {
		if err := fw.FilterForwardDrop(context.TODO(), firewaller.IPv6); err != nil {
			return err
		}
	}

	return nil
}

func configureIPForwarding(file string, val byte) (changed bool, _ error) {
	data, err := os.ReadFile(file)
	if err != nil || len(data) == 0 {
		return false, fmt.Errorf("cannot read IP forwarding setup from '%s': %w", file, err)
	}
	if len(data) == 0 {
		return false, fmt.Errorf("cannot read IP forwarding setup from '%s': 0 bytes", file)
	}
	if data[0] == val {
		return false, nil
	}
	if err := os.WriteFile(file, []byte{val, '\n'}, 0o644); err != nil {
		return false, fmt.Errorf("failed to set IP forwarding '%s' = '%c': %w", file, val, err)
	}
	return true, nil
}
