package netlabel

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
)
