//go:build linux

package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/modprobe"
	"golang.org/x/sys/unix"
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
		return errors.New("unable to check IPv6 forwarding, no bridge name specified")
	}
	if enabled, err := getKernelBoolParam("/proc/sys/net/ipv6/conf/" + config.BridgeName + "/forwarding"); err != nil {
		log.G(context.TODO()).Warnf("failed to check IPv6 forwarding: %v", err)
		return nil
	} else if enabled {
		return enableBridgeNetFiltering("/proc/sys/net/bridge/bridge-nf-call-ip6tables")
	}
	return nil
}

// brNetfilterUsable returns nil if br_netfilter can be used for the IP versions
// that need it. That's if the module is loaded (or can be loaded), and the
// bridge-nf-call-ip[6]tables sysctls exist and are either already enabled, or
// can be written.
//
// br_netfilter is deprecated, and it's disabled in user namespaces from kernel
// 7.4. Then, the sysctls don't exist.
func brNetfilterUsable(ipv4, ipv6 bool) error {
	return brNetfilterUsableAt("/proc/sys/net/bridge/bridge-nf-call-iptables",
		"/proc/sys/net/bridge/bridge-nf-call-ip6tables", ipv4, ipv6)
}

func brNetfilterUsableAt(param4, param6 string, ipv4, ipv6 bool) error {
	if !ipv4 && !ipv6 {
		return nil
	}
	if err := loadBridgeNetFilterModule(param4); err != nil {
		return fmt.Errorf("loading br_netfilter: %w", err)
	}
	for _, p := range []struct {
		need bool
		path string
	}{{ipv4, param4}, {ipv6, param6}} {
		if !p.need {
			continue
		}
		enabled, err := getKernelBoolParam(p.path)
		if err != nil {
			return err
		}
		if enabled {
			continue
		}
		if err := unix.Access(p.path, unix.W_OK); err != nil {
			return fmt.Errorf("%s is not enabled, and cannot be written: %w", p.path, err)
		}
	}
	return nil
}

func loadBridgeNetFilterModule(fullPath string) error {
	// br_netfilter implicitly loads bridge module upon modprobe
	return modprobe.LoadModules(context.TODO(), func() error {
		_, err := os.Stat(fullPath)
		return err
	}, "br_netfilter")
}

// Enable bridge net filtering if not already enabled. See GitHub issue #11404
func enableBridgeNetFiltering(nfParam string) (retErr error) {
	defer func() {
		if retErr != nil {
			if os.Getenv("DOCKER_IGNORE_BR_NETFILTER_ERROR") == "1" {
				log.G(context.TODO()).WithError(retErr).Warnf("Continuing without enabling br_netfilter")
				retErr = nil
				return
			}
			retErr = fmt.Errorf("%w: set environment variable DOCKER_IGNORE_BR_NETFILTER_ERROR=1 to ignore", retErr)
		}
	}()

	if err := loadBridgeNetFilterModule(nfParam); err != nil {
		return fmt.Errorf("cannot restrict inter-container communication or run without the userland proxy: %w", err)
	}
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
		return fmt.Errorf("cannot restrict inter-container communication or run without the userland proxy: %v", err)
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
