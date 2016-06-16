package windows

const (
	// NetworkName label for bridge driver
	NetworkName = "com.docker.network.windowsshim.networkname"

	// HNSID of the discovered network
	HNSID = "com.docker.network.windowsshim.hnsid"

	// RoutingDomain of the network
	RoutingDomain = "com.docker.network.windowsshim.routingdomain"

	// Interface of the network
	Interface = "com.docker.network.windowsshim.interface"

	// QosPolicies of the endpoint
	QosPolicies = "com.docker.endpoint.windowsshim.qospolicies"
)
