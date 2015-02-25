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
//    networkNamespace, err := libnetwork.NewNamespace(networkPath)
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

// A Network represents a logical connectivity zone that containers may
// ulteriorly join using the Link method. A Network is managed by a specific
// driver.
type Network interface {
	Type() string
	Link(name string) ([]*Interface, error)
}

type Interface struct {
	// The name of the interface in the origin network namespace.
	SrcName string

	// The name that will be assigned to the interface once moves inside a
	// network namespace.
	DstName string

	MacAddress  string
	Address     string
	AddressIPv6 string
	Gateway     string
	GatewayIPv6 string
	MTU         int
}

type Namespace interface {
	Path() string
	Interfaces() []*Interface
	AddInterface(*Interface) error
}

func NewNetwork(networkType string, options DriverParams) (Network, error) {
	return createNetwork(networkType, options)
}
