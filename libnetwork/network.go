// Package libnetwork provides basic fonctionalities and extension points to
// create network namespaces and allocate interfaces for containers to use.
//
//    // Create a network for containers to join.
//    network, err := libnetwork.NewNetwork("simplebridge", &Options{})
//    if err != nil {
//    	return err
//    }
//
//    // For a new container: create network namespace (providing the path).
//    networkPath := "/var/lib/docker/.../4d23e"
//    networkNamespace, err := libnetwork.NewNetworkNamespace(networkPath)
//    if err != nil {
//    	return err
//    }
//
//    // For each new container: allocate IP and interfaces. The returned network
//    // settings will be used for container infos (inspect and such), as well as
//    // iptables rules for port publishing.
//    interfaces, err := network.Link(containerID)
//    if err != nil {
//    	return err
//    }
//
//    // Add interfaces to the namespace.
//    for _, interface := range interfaces {
//    	if err := networkNamespace.AddInterface(interface); err != nil {
//    		return err
//    	}
//    }
//
package libnetwork

// Interface represents the settings and identity of a network device. It is
// used as a return type for Network.Link, and it is common practice for the
// caller to use this information when moving interface SrcName from host
// namespace to DstName in a different net namespace with the appropriate
// network settings.
type Interface struct {
	// The name of the interface in the origin network namespace.
	SrcName string

	// The name that will be assigned to the interface once moves inside a
	// network namespace.
	DstName string

	// MAC address for the interface.
	MacAddress string

	// IPv4 address for the interface.
	Address string

	// IPv6 address for the interface.
	AddressIPv6 string

	// IPv4 gateway for the interface.
	Gateway string

	// IPv6 gateway for the interface.
	GatewayIPv6 string

	// Network MTU.
	MTU int
}

// A Network represents a logical connectivity zone that containers may
// ulteriorly join using the Link method. A Network is managed by a specific
// driver.
type Network interface {
	// A user chosen name for this network.
	Name() string

	// The type of network, which corresponds to its managing driver.
	Type() string

	// Create a new link to this network symbolically identified by the
	// specified unique name.
	Link(name string) ([]*Interface, error)
}

// Namespace represents a network namespace, mounted on a specific Path.  It
// holds a list of Interface, and more can be added dynamically.
type Namespace interface {
	// The path where the network namespace is mounted.
	Path() string

	// The collection of Interface previously added with the AddInterface
	// method. Note that this doesn't incude network interfaces added in any
	// other way (such as the default loopback interface existing in any newly
	// created network namespace).
	Interfaces() []*Interface

	// Add an existing Interface to this namespace. The operation will rename
	// from the Interface SrcName to DstName as it moves, and reconfigure the
	// interface according to the specified settings.
	AddInterface(*Interface) error
}

// Create a new network of the specified networkType. The options are driver
// specific and modeled in a generic way.
func NewNetwork(networkType, name string, options DriverParams) (Network, error) {
	return createNetwork(networkType, name, options)
}

// Create a new network namespace mounted on the specified path.
func NewNetworkNamespace(path string) (Namespace, error) {
	return createNetworkNamespace(path)
}
