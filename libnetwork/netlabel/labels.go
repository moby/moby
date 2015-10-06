package netlabel

import (
	"fmt"
	"strings"
)

const (
	// Prefix constant marks the reserved label space for libnetwork
	Prefix = "com.docker.network"

	// DriverPrefix constant marks the reserved label space for libnetwork drivers
	DriverPrefix = Prefix + ".driver"

	// DriverPrivatePrefix constant marks the reserved label space
	// for internal libnetwork drivers
	DriverPrivatePrefix = DriverPrefix + ".private"

	// GenericData constant that helps to identify an option as a Generic constant
	GenericData = Prefix + ".generic"

	// PortMap constant represents Port Mapping
	PortMap = Prefix + ".portmap"

	// MacAddress constant represents Mac Address config of a Container
	MacAddress = Prefix + ".endpoint.macaddress"

	// ExposedPorts constant represents the container's Exposed Ports
	ExposedPorts = Prefix + ".endpoint.exposedports"

	//EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// OverlayBindInterface constant represents overlay driver bind interface
	OverlayBindInterface = DriverPrefix + ".overlay.bind_interface"

	// OverlayNeighborIP constant represents overlay driver neighbor IP
	OverlayNeighborIP = DriverPrefix + ".overlay.neighbor_ip"

	// Gateway represents the gateway for the network
	Gateway = Prefix + ".gateway"
)

var (
	// GlobalKVProvider constant represents the KV provider backend
	GlobalKVProvider = MakeKVProvider("global")

	// GlobalKVProviderURL constant represents the KV provider URL
	GlobalKVProviderURL = MakeKVProviderURL("global")

	// GlobalKVProviderConfig constant represents the KV provider Config
	GlobalKVProviderConfig = MakeKVProviderConfig("global")

	// LocalKVProvider constant represents the KV provider backend
	LocalKVProvider = MakeKVProvider("local")

	// LocalKVProviderURL constant represents the KV provider URL
	LocalKVProviderURL = MakeKVProviderURL("local")

	// LocalKVProviderConfig constant represents the KV provider Config
	LocalKVProviderConfig = MakeKVProviderConfig("local")
)

// MakeKVProvider returns the kvprovider label for the scope
func MakeKVProvider(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider"
}

// MakeKVProviderURL returns the kvprovider url label for the scope
func MakeKVProviderURL(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider_url"
}

// MakeKVProviderConfig returns the kvprovider config label for the scope
func MakeKVProviderConfig(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider_config"
}

// Key extracts the key portion of the label
func Key(label string) string {
	kv := strings.SplitN(label, "=", 2)

	return kv[0]
}

// Value extracts the value portion of the label
func Value(label string) string {
	kv := strings.SplitN(label, "=", 2)

	return kv[1]
}

// KeyValue decomposes the label in the (key,value) pair
func KeyValue(label string) (string, string, error) {
	kv := strings.SplitN(label, "=", 2)
	if len(kv) != 2 {
		return "", "", fmt.Errorf("invalid label: %s", label)
	}
	return kv[0], kv[1], nil
}

// ToMap converts a list of labels in amap of (key,value) pairs
func ToMap(labels []string) map[string]string {
	m := make(map[string]string, len(labels))
	for _, l := range labels {
		if k, v, err := KeyValue(l); err == nil {
			m[k] = v
		}
	}
	return m
}
