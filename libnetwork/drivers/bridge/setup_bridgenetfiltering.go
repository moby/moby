//go:build linux

package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/containerd/log"
)

// setupIPv4BridgeNetFiltering checks whether IPv4 forwarding is enabled and, if
// it is, sets kernel param "bridge-nf-call-iptables=1" so that packets traversing
// the bridge are filtered.
func setupIPv4BridgeNetFiltering(*networkConfiguration, *bridgeInterface) error {
	if enabled, err := getKernelBoolParam("/proc/sys/net/ipv4/ip_forward"); err != nil {
		log.G(context.TODO()).Warnf("failed to check IPv4 forwarding: %v", err)
		return nil
	} else if enabled {
		return enableBridgeNetFiltering("/proc/sys/net/bridge/bridge-nf-call-iptables")
	}
	return nil
}

// setupIPv6BridgeNetFiltering checks whether IPv6 forwarding is enabled for the
// bridge and, if it is, sets kernel param "bridge-nf-call-ip6tables=1" so that
// packets traversing the bridge are filtered.
func setupIPv6BridgeNetFiltering(config *networkConfiguration, _ *bridgeInterface) error {
	if !config.EnableIPv6 {
		return nil
	}
	if config.BridgeName == "" {
		return fmt.Errorf("unable to check IPv6 forwarding, no bridge name specified")
	}
	if enabled, err := getKernelBoolParam("/proc/sys/net/ipv6/conf/" + config.BridgeName + "/forwarding"); err != nil {
		log.G(context.TODO()).Warnf("failed to check IPv6 forwarding: %v", err)
		return nil
	} else if enabled {
		return enableBridgeNetFiltering("/proc/sys/net/bridge/bridge-nf-call-ip6tables")
	}
	return nil
}

// Enable bridge net filtering if not already enabled. See GitHub issue #11404
func enableBridgeNetFiltering(nfParam string) error {
	enabled, err := getKernelBoolParam(nfParam)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && errors.Is(pathErr, syscall.ENOENT) {
			if isRunningInContainer() {
				log.G(context.TODO()).WithError(err).Warnf("running inside docker container, ignoring missing kernel params")
				return nil
			}
			err = errors.New("ensure that the br_netfilter kernel module is loaded")
		}
		return fmt.Errorf("cannot restrict inter-container communication: %v", err)
	}
	if !enabled {
		return os.WriteFile(nfParam, []byte{'1', '\n'}, 0o644)
	}
	return nil
}

// Gets the value of the kernel parameters located at the given path
func getKernelBoolParam(path string) (bool, error) {
	line, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return len(line) > 0 && line[0] == '1', nil
}

func isRunningInContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return !os.IsNotExist(err)
}
