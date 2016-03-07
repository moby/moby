package bridge

const (
	// BridgeName label for bridge driver
	BridgeName = "com.docker.network.bridge.name"

	// EnableIPMasquerade label for bridge driver
	EnableIPMasquerade = "com.docker.network.bridge.enable_ip_masquerade"

	// EnableICC label
	EnableICC = "com.docker.network.bridge.enable_icc"

	// DefaultBindingIP label
	DefaultBindingIP = "com.docker.network.bridge.host_binding_ipv4"

	// DefaultBridge label
	DefaultBridge = "com.docker.network.bridge.default_bridge"

	// ChildEndpoints for links
	ChildEndpoints = "com.docker.network.bridge.child_endpoints"

	// ParentEndpoints for links
	ParentEndpoints = "com.docker.network.bridge.parent_endpoints"
)
