package runconfig

const (
	// ErrConflictContainerNetworkAndLinks conflict between --net=container and links
	ErrConflictContainerNetworkAndLinks validationError = "conflicting options: container type network can't be used with links. This would result in undefined behavior"
	// ErrConflictSharedNetwork conflict between private and other networks
	ErrConflictSharedNetwork validationError = "container sharing network namespace with another container or host cannot be connected to any other network"
	// ErrConflictHostNetwork conflict from being disconnected from host network or connected to host network.
	ErrConflictHostNetwork validationError = "container cannot be disconnected from host network or connected to host network"
	// ErrConflictNoNetwork conflict between private and other networks
	ErrConflictNoNetwork validationError = "container cannot be connected to multiple networks with one of the networks in private (none) mode"
	// ErrConflictNetworkAndDNS conflict between --dns and the network mode
	ErrConflictNetworkAndDNS validationError = "conflicting options: dns and the network mode"
	// ErrConflictNetworkHostname conflict between the hostname and the network mode
	ErrConflictNetworkHostname validationError = "conflicting options: hostname and the network mode"
	// ErrConflictHostNetworkAndLinks conflict between --net=host and links
	ErrConflictHostNetworkAndLinks validationError = "conflicting options: host type networking can't be used with links. This would result in undefined behavior"
	// ErrConflictContainerNetworkAndMac conflict between the mac address and the network mode
	ErrConflictContainerNetworkAndMac validationError = "conflicting options: mac-address and the network mode"
	// ErrConflictNetworkHosts conflict between add-host and the network mode
	ErrConflictNetworkHosts validationError = "conflicting options: custom host-to-IP mapping and the network mode"
	// ErrConflictNetworkPublishPorts conflict between the publish options and the network mode
	ErrConflictNetworkPublishPorts validationError = "conflicting options: port publishing and the container type network mode"
	// ErrConflictNetworkExposePorts conflict between the expose option and the network mode
	ErrConflictNetworkExposePorts validationError = "conflicting options: port exposing and the container type network mode"
	// ErrUnsupportedNetworkAndIP conflict between network mode and requested ip address
	ErrUnsupportedNetworkAndIP validationError = "user specified IP address is supported on user defined networks only"
	// ErrUnsupportedNetworkNoSubnetAndIP conflict between network with no configured subnet and requested ip address
	ErrUnsupportedNetworkNoSubnetAndIP validationError = "user specified IP address is supported only when connecting to networks with user configured subnets"
	// ErrUnsupportedNetworkAndAlias conflict between network mode and alias
	ErrUnsupportedNetworkAndAlias validationError = "network-scoped alias is supported only for containers in user defined networks"
	// ErrConflictUTSHostname conflict between the hostname and the UTS mode
	ErrConflictUTSHostname validationError = "conflicting options: hostname and the UTS mode"
)

type validationError string

func (e validationError) Error() string {
	return string(e)
}

func (e validationError) InvalidParameter() {}
