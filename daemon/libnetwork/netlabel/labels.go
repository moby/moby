package netlabel

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

	// EndpointSysctls is a comma separated list interface-specific sysctls
	// where the interface name is represented by the string "IFNAME".
	EndpointSysctls = Prefix + ".endpoint.sysctls"

	// Ifname can be used to set the interface name used inside the container. It takes precedence over ContainerIfacePrefix.
	Ifname = Prefix + ".endpoint.ifname"

	// EnableIPv4 constant represents enabling IPV4 at network level
	EnableIPv4 = Prefix + ".enable_ipv4"

	// EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// DriverMTU constant represents the MTU size for the network driver
	DriverMTU = DriverPrefix + ".mtu"

	// AdvertiseAddrNMsgs is the number of unsolicited ARP/NA messages that will be sent to
	// advertise an interface's IP and MAC addresses.
	AdvertiseAddrNMsgs = Prefix + ".advertise_addr_nmsgs"

	// AdvertiseAddrIntervalMs is the minimum interval between ARP/NA advertisements for
	// an interface's IP and MAC addresses (in milliseconds).
	AdvertiseAddrIntervalMs = Prefix + ".advertise_addr_ms"

	// OverlayVxlanIDList constant represents a list of VXLAN Ids as csv
	OverlayVxlanIDList = DriverPrefix + ".overlay.vxlanid_list"

	// Gateway represents the gateway for the network
	Gateway = Prefix + ".gateway"

	// Internal constant represents that the network is internal which disables default gateway service
	Internal = Prefix + ".internal"

	// ContainerIfacePrefix can be used to override the interface prefix used inside the container
	ContainerIfacePrefix = Prefix + ".container_iface_prefix"

	// HostIPv4 is the Source-IPv4 Address used to SNAT IPv4 container traffic
	HostIPv4 = Prefix + ".host_ipv4"

	// HostIPv6 is the Source-IPv6 Address used to SNAT IPv6 container traffic
	HostIPv6 = Prefix + ".host_ipv6"

	// NoProxy6To4 disables proxying from an IPv6 host port to an IPv4-only
	// container, when the default binding address is 0.0.0.0. This label
	// is intended for internal use, it may be removed in a future release.
	NoProxy6To4 = DriverPrivatePrefix + ".no_proxy_6to4"
)

// GetIfname returns the value associated to the Ifname netlabel from the
// provided options. If there's no Ifname netlabel, or if the value isn't a
// string, it returns an empty string.
func GetIfname(opts map[string]any) string {
	ifname, _ := opts[Ifname].(string)
	return ifname
}
