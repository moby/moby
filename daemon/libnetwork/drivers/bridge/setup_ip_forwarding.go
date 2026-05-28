//go:build linux

package bridge

import (
	"context"
	"errors"
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

func checkIPv4Forwarding() error {
	enabled, err := getKernelBoolParam(ipv4ForwardConf)
	if err != nil {
		return fmt.Errorf("checking IPv4 forwarding: %w", err)
	}
	if enabled {
		return nil
	}
	// It's the user's responsibility to enable forwarding and secure their host. Or,
	// start docker with --ip-forward=false to disable this check.
	return errors.New("IPv4 forwarding is disabled: check your host's firewalling and set sysctl net.ipv4.ip_forward=1, or disable this check using daemon option --ip-forward=false")
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
		if err := ffd.FilterForwardDrop(context.TODO(), firewaller.IPv4); err != nil {
			return err
		}
	}
	return nil
}

func checkIPv6Forwarding() error {
	enabledDef, err := getKernelBoolParam(ipv6ForwardConfDefault)
	if err != nil {
		return fmt.Errorf("checking IPv6 default forwarding: %w", err)
	}
	enabledAll, err := getKernelBoolParam(ipv6ForwardConfAll)
	if err != nil {
		return fmt.Errorf("checking IPv6 global forwarding: %w", err)
	}
	if enabledDef && enabledAll {
		return nil
	}

	// It's the user's responsibility to enable forwarding and secure their host. Or,
	// start docker with --ip-forward=false to disable this check.
	return errors.New("IPv6 global forwarding is disabled: check your host's firewalling and set sysctls net.ipv6.conf.all.forwarding=1 and net.ipv6.conf.default.forwarding=1, or disable this check using daemon option --ip-forward=false")
}

func setupIPv6Forwarding(ffd filterForwardDropper, wantFilterForwardDrop bool) (retErr error) {
	// Set IPv6 default.forwarding, if needed.
	// Setting "all" (below) sets "default" as well, but need to check that "default" is
	// set even if "all" is already set.
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
		if err := ffd.FilterForwardDrop(context.TODO(), firewaller.IPv6); err != nil {
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
