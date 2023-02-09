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

	// EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// DriverMTU constant represents the MTU size for the network driver
	DriverMTU = DriverPrefix + ".mtu"

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

	// LocalKVClient constants represents the local kv store client
	LocalKVClient = DriverPrivatePrefix + "localkv_client"
)
