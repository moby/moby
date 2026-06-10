//go:build !linux

package bridge

// Configuration holds bridge driver settings.
type Configuration struct {
	EnableIPForwarding       bool
	DisableFilterForwardDrop bool
	EnableIPTables           bool
	EnableIP6Tables          bool
	EnableProxy              bool
	ProxyPath                string
	AllowDirectRouting       bool
	AcceptFwMark             string
}

const (
	NetworkType = "bridge"

	DefaultBridgeName      = "bridge"
	DefaultGatewayV4AuxKey = "com.docker.network.bridge.default_gateway_v4"
	DefaultGatewayV6AuxKey = "com.docker.network.bridge.default_gateway_v6"
)

// LegacyContainerLinkOptions returns labels for legacy container links.
func LegacyContainerLinkOptions(_, _ []string) map[string]string {
	return nil
}

// ValidateFixedCIDRV6 validates the provided IPv6 CIDR.
func ValidateFixedCIDRV6(_ string) error {
	return nil
}
