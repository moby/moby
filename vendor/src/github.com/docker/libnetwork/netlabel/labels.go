package netlabel

import "strings"

const (
	// Prefix constant marks the reserved label space for libnetwork
	Prefix = "com.docker.network"

	// DriverPrefix constant marks the reserved label space for libnetwork drivers
	DriverPrefix = Prefix + ".driver"

	// GenericData constant that helps to identify an option as a Generic constant
	GenericData = Prefix + ".generic"

	// PortMap constant represents Port Mapping
	PortMap = Prefix + ".portmap"

	// MacAddress constant represents Mac Address config of a Container
	MacAddress = Prefix + ".endpoint.macaddress"

	// ExposedPorts constant represents exposedports of a Container
	ExposedPorts = Prefix + ".endpoint.exposedports"

	//EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// KVProvider constant represents the KV provider backend
	KVProvider = DriverPrefix + ".kv_provider"

	// KVProviderURL constant represents the KV provider URL
	KVProviderURL = DriverPrefix + ".kv_provider_url"

	// KVProviderConfig constant represents the KV provider Config
	KVProviderConfig = DriverPrefix + ".kv_provider_config"

	// OverlayBindInterface constant represents overlay driver bind interface
	OverlayBindInterface = DriverPrefix + ".overlay.bind_interface"

	// OverlayNeighborIP constant represents overlay driver neighbor IP
	OverlayNeighborIP = DriverPrefix + ".overlay.neighbor_ip"
)

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
