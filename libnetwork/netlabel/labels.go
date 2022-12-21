package netlabel

import (
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

	// DNSServers A list of DNS servers associated with the endpoint
	DNSServers = Prefix + ".endpoint.dnsservers"

	// EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// DriverMTU constant represents the MTU size for the network driver
	DriverMTU = DriverPrefix + ".mtu"

	// OverlayBindInterface constant represents overlay driver bind interface
	OverlayBindInterface = DriverPrefix + ".overlay.bind_interface"

	// OverlayNeighborIP constant represents overlay driver neighbor IP
	OverlayNeighborIP = DriverPrefix + ".overlay.neighbor_ip"

	// OverlayVxlanIDList constant represents a list of VXLAN Ids as csv
	OverlayVxlanIDList = DriverPrefix + ".overlay.vxlanid_list"

	// Gateway represents the gateway for the network
	Gateway = Prefix + ".gateway"

	// Internal constant represents that the network is internal which disables default gateway service
	Internal = Prefix + ".internal"

	// ContainerIfacePrefix can be used to override the interface prefix used inside the container
	ContainerIfacePrefix = Prefix + ".container_iface_prefix"

	// HostIP is the Source-IP Address used to SNAT container traffic
	HostIP = Prefix + ".host_ipv4"

	// GlobalKVProvider constant represents the KV provider backend
	GlobalKVProvider = DriverPrivatePrefix + "globalkv_provider"

	// GlobalKVProviderURL constant represents the KV provider URL
	GlobalKVProviderURL = DriverPrivatePrefix + "globalkv_provider_url"

	// GlobalKVProviderConfig constant represents the KV provider Config
	GlobalKVProviderConfig = DriverPrivatePrefix + "globalkv_provider_config"

	// GlobalKVClient constants represents the global kv store client
	GlobalKVClient = DriverPrivatePrefix + "globalkv_client"

	// LocalKVProvider constant represents the KV provider backend
	LocalKVProvider = DriverPrivatePrefix + "localkv_provider"

	// LocalKVProviderURL constant represents the KV provider URL
	LocalKVProviderURL = DriverPrivatePrefix + "localkv_provider_url"

	// LocalKVProviderConfig constant represents the KV provider Config
	LocalKVProviderConfig = DriverPrivatePrefix + "localkv_provider_config"

	// LocalKVClient constants represents the local kv store client
	LocalKVClient = DriverPrivatePrefix + "localkv_client"
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

// MakeKVClient returns the kv client label for the scope
func MakeKVClient(scope string) string {
	return DriverPrivatePrefix + scope + "kv_client"
}

// Key extracts the key portion of the label
func Key(label string) (key string) {
	key, _, _ = strings.Cut(label, "=")
	return key
}

// Value extracts the value portion of the label
func Value(label string) (value string) {
	_, value, _ = strings.Cut(label, "=")
	return value
}

// KeyValue decomposes the label in the (key,value) pair
func KeyValue(label string) (key string, value string) {
	key, value, _ = strings.Cut(label, "=")
	return key, value
}
