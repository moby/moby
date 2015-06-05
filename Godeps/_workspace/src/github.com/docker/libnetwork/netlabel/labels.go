package netlabel

const (
	// GenericData constant that helps to identify an option as a Generic constant
	GenericData = "io.docker.network.generic"

	// PortMap constant represents Port Mapping
	PortMap = "io.docker.network.endpoint.portmap"

	// MacAddress constant represents Mac Address config of a Container
	MacAddress = "io.docker.network.endpoint.macaddress"

	// ExposedPorts constant represents exposedports of a Container
	ExposedPorts = "io.docker.network.endpoint.exposedports"

	//EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = "io.docker.network.enable_ipv6"
)
