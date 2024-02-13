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

// Enumeration type saying which versions of IP protocol to process.
type ipVersion int

const (
	ipvnone ipVersion = iota
	ipv4
	ipv6
	ipvboth
)

// getIPVersion gets the IP version in use ( [ipv4], [ipv6] or [ipv4 and ipv6] )
func getIPVersion(config *networkConfiguration) ipVersion {
	ipVer := ipv4
	if config.AddressIPv6 != nil || config.EnableIPv6 {
		ipVer |= ipv6
	}
	return ipVer
}

func setupBridgeNetFiltering(config *networkConfiguration, _ *bridgeInterface) error {
	if err := checkBridgeNetFiltering(config); err != nil {
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
	return nil
}

// Enable bridge net filtering if ip forwarding is enabled. See github issue #11404
func checkBridgeNetFiltering(config *networkConfiguration) error {
	ipVer := getIPVersion(config)
	iface := config.BridgeName
	doEnable := func(ipVer ipVersion) error {
		enabled, err := isPacketForwardingEnabled(ipVer, iface)
		if err != nil {
			var ipVerName string
			if ipVer == ipv4 {
				ipVerName = "IPv4"
			} else {
				ipVerName = "IPv6"
			}
			log.G(context.TODO()).Warnf("failed to check %s forwarding: %v", ipVerName, err)
		} else if enabled {
			enabled, err := getKernelBoolParam(getBridgeNFKernelParam(ipVer))
			if err != nil || enabled {
				return err
			}
			return setKernelBoolParam(getBridgeNFKernelParam(ipVer), true)
		}
		return nil
	}

	switch ipVer {
	case ipv4, ipv6:
		return doEnable(ipVer)
	case ipvboth:
		v4err := doEnable(ipv4)
		v6err := doEnable(ipv6)
		if v4err == nil {
			return v6err
		}
		return v4err
	default:
		return nil
	}
}

// Get kernel param path saying whether IPv${ipVer} traffic is being forwarded
// on particular interface. Interface may be specified for IPv6 only. If
// `iface` is empty, `default` will be assumed, which represents default value
// for new interfaces.
func getForwardingKernelParam(ipVer ipVersion, iface string) string {
	switch ipVer {
	case ipv4:
		return "/proc/sys/net/ipv4/ip_forward"
	case ipv6:
		if iface == "" {
			iface = "default"
		}
		return fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/forwarding", iface)
	default:
		return ""
	}
}

// Get kernel param path saying whether bridged IPv${ipVer} traffic shall be
// passed to ip${ipVer}tables' chains.
func getBridgeNFKernelParam(ipVer ipVersion) string {
	switch ipVer {
	case ipv4:
		return "/proc/sys/net/bridge/bridge-nf-call-iptables"
	case ipv6:
		return "/proc/sys/net/bridge/bridge-nf-call-ip6tables"
	default:
		return ""
	}
}

// Gets the value of the kernel parameters located at the given path
func getKernelBoolParam(path string) (bool, error) {
	enabled := false
	line, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(line) > 0 {
		enabled = line[0] == '1'
	}
	return enabled, err
}

// Sets the value of the kernel parameter located at the given path
func setKernelBoolParam(path string, on bool) error {
	value := byte('0')
	if on {
		value = byte('1')
	}
	return os.WriteFile(path, []byte{value, '\n'}, 0o644)
}

// Checks to see if packet forwarding is enabled
func isPacketForwardingEnabled(ipVer ipVersion, iface string) (bool, error) {
	switch ipVer {
	case ipv4, ipv6:
		return getKernelBoolParam(getForwardingKernelParam(ipVer, iface))
	case ipvboth:
		enabled, err := getKernelBoolParam(getForwardingKernelParam(ipv4, ""))
		if err != nil || !enabled {
			return enabled, err
		}
		return getKernelBoolParam(getForwardingKernelParam(ipv6, iface))
	default:
		return true, nil
	}
}

func isRunningInContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return !os.IsNotExist(err)
}
