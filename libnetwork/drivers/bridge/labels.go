package bridge

const (
	// BridgeName label for bridge driver
	BridgeName = "com.docker.network.bridge.name"

	// EnableIPMasquerade label for bridge driver
	EnableIPMasquerade = "com.docker.network.bridge.enable_ip_masquerade"

	// IPv4GatewayMode label for bridge driver
	IPv4GatewayMode = "com.docker.network.bridge.gateway_mode_ipv4"
	// IPv6GatewayMode label for bridge driver
	IPv6GatewayMode = "com.docker.network.bridge.gateway_mode_ipv6"

	// EnableICC label
	EnableICC = "com.docker.network.bridge.enable_icc"

	// InhibitIPv4 label
	InhibitIPv4 = "com.docker.network.bridge.inhibit_ipv4"

	// DefaultBindingIP label
	DefaultBindingIP = "com.docker.network.bridge.host_binding_ipv4"

	// DefaultBridge label
	DefaultBridge = "com.docker.network.bridge.default_bridge"
)
