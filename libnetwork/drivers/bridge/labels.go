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
	DefaultBindingIP = "com.docker.network.bridge.host_binding_ip"

	// DefaultBindingIPv4 label
	// 'host_binding_ipv4' existed before 'host_binding_ip', despite the name
	// it accepted IPv6 addresses. The options are now synonyms, and it is an
	// error to specify both.
	DefaultBindingIPv4 = "com.docker.network.bridge.host_binding_ipv4"

	// DefaultBridge label
	DefaultBridge = "com.docker.network.bridge.default_bridge"
)
