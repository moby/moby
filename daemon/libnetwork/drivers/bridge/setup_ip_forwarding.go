//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
)

const (
	ipv4ForwardConf        = "/proc/sys/net/ipv4/ip_forward"
	ipv6ForwardConfDefault = "/proc/sys/net/ipv6/conf/default/forwarding"
	ipv6ForwardConfAll     = "/proc/sys/net/ipv6/conf/all/forwarding"
)

type filterForwardDropper interface {
	FilterForwardDrop(context.Context, firewaller.IPVersion) error
}

func setupIPv4Forwarding(ffd filterForwardDropper, wantFilterForwardDrop bool) (retErr error) {
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
		if err := filterForwardDrop(context.TODO(), ffd, firewaller.IPv4); err != nil {
			return err
		}
	}
	return nil
}

func setupIPv6Forwarding(ffd filterForwardDropper, wantFilterForwardDrop bool) (retErr error) {
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
		if err := filterForwardDrop(context.TODO(), ffd, firewaller.IPv6); err != nil {
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

func filterForwardDrop(ctx context.Context, ffd filterForwardDropper, ipv firewaller.IPVersion) error {
	if ffd == nil {
		log.G(ctx).WithField("ipv", ipv).Warn("Enabled IP forwarding in the kernel. If necessary, make sure the host has firewall rules to block forwarding between non-Docker network interfaces.")
		return nil
	}
	return ffd.FilterForwardDrop(ctx, ipv)
}
