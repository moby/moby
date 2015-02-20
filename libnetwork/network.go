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
//    interfaces, err := network.CreateInterfaces(containerID)
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
package libnetwork

import "fmt"

type Network interface {
	Name() string
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

// TODO Figure out the proper options type
func NewNetwork(networkType string, options strategyParams) (Network, error) {
	if ctor, ok := strategies[networkType]; ok {
		return ctor(options)
	}
	return nil, fmt.Errorf("Unknown network type %q", networkType)
}
