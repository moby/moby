package client

import "github.com/docker/libnetwork/types"

/***********
 Resources
************/

// networkResource is the body of the "get network" http response message
type networkResource struct {
	Name     string             `json:"name"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Services []*serviceResource `json:"services"`
}

// serviceResource is the body of the "get service" http response message
type serviceResource struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Network string `json:"network"`
}

// backendResource is the body of "get service backend" response message
type backendResource struct {
	ID string `json:"id"`
}

/***********
  Body types
  ************/

// networkCreate is the expected body of the "create network" http request message
type networkCreate struct {
	Name        string                 `json:"name"`
	NetworkType string                 `json:"network_type"`
	Options     map[string]interface{} `json:"options"`
}

// serviceCreate represents the body of the "publish service" http request message
type serviceCreate struct {
	Name         string                `json:"name"`
	Network      string                `json:"network_name"`
	ExposedPorts []types.TransportPort `json:"exposed_ports"`
	PortMapping  []types.PortBinding   `json:"port_mapping"`
}

// serviceAttach represents the expected body of the "attach/detach backend to/from service" http request messages
type serviceAttach struct {
	ContainerID       string                `json:"container_id"`
	HostName          string                `json:"host_name"`
	DomainName        string                `json:"domain_name"`
	HostsPath         string                `json:"hosts_path"`
	ResolvConfPath    string                `json:"resolv_conf_path"`
	DNS               []string              `json:"dns"`
	ExtraHosts        []serviceExtraHost    `json:"extra_hosts"`
	ParentUpdates     []serviceParentUpdate `json:"parent_updates"`
	UseDefaultSandbox bool                  `json:"use_default_sandbox"`
}

// serviceExtraHost represents the extra host object
type serviceExtraHost struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// EndpointParentUpdate is the object carrying the information about the
// endpoint parent that needs to be updated
type serviceParentUpdate struct {
	EndpointID string `json:"service_id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
}
