package api

import "github.com/docker/libnetwork/netutils"

/***********
 Resources
************/

// networkResource is the body of the "get network" http response message
type networkResource struct {
	Name      string
	ID        string
	Type      string
	Endpoints []*endpointResource
}

// endpointResource is the body of the "get endpoint" http response message
type endpointResource struct {
	Name    string
	ID      string
	Network string
}

/***********
  Body types
  ************/

// networkCreate is the expected body of the "create network" http request message
type networkCreate struct {
	Name        string
	NetworkType string
	Options     map[string]interface{}
}

// endpointCreate represents the body of the "create endpoint" http request message
type endpointCreate struct {
	Name         string
	NetworkID    string
	ExposedPorts []netutils.TransportPort
	PortMapping  []netutils.PortBinding
}

// endpointJoin represents the expected body of the "join endpoint" or "leave endpoint" http request messages
type endpointJoin struct {
	ContainerID       string
	HostName          string
	DomainName        string
	HostsPath         string
	ResolvConfPath    string
	DNS               []string
	ExtraHosts        []endpointExtraHost
	ParentUpdates     []endpointParentUpdate
	UseDefaultSandbox bool
}

// EndpointExtraHost represents the extra host object
type endpointExtraHost struct {
	Name    string
	Address string
}

// EndpointParentUpdate is the object carrying the information about the
// endpoint parent that needs to be updated
type endpointParentUpdate struct {
	EndpointID string
	Name       string
	Address    string
}
