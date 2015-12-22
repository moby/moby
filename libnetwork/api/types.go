package api

import "github.com/docker/libnetwork/types"

/***********
 Resources
************/

// networkResource is the body of the "get network" http response message
type networkResource struct {
	Name      string              `json:"name"`
	ID        string              `json:"id"`
	Type      string              `json:"type"`
	Endpoints []*endpointResource `json:"endpoints"`
}

// endpointResource is the body of the "get endpoint" http response message
type endpointResource struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Network string `json:"network"`
}

// sandboxResource is the body of "get service backend" response message
type sandboxResource struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	ContainerID string `json:"container_id"`
}

/***********
  Body types
  ************/

// networkCreate is the expected body of the "create network" http request message
type networkCreate struct {
	Name        string            `json:"name"`
	NetworkType string            `json:"network_type"`
	DriverOpts  map[string]string `json:"driver_opts"`
	NetworkOpts map[string]string `json:"network_opts"`
}

// endpointCreate represents the body of the "create endpoint" http request message
type endpointCreate struct {
	Name         string                `json:"name"`
	ExposedPorts []types.TransportPort `json:"exposed_ports"`
	PortMapping  []types.PortBinding   `json:"port_mapping"`
}

// sandboxCreate is the expected body of the "create sandbox" http request message
type sandboxCreate struct {
	ContainerID       string      `json:"container_id"`
	HostName          string      `json:"host_name"`
	DomainName        string      `json:"domain_name"`
	HostsPath         string      `json:"hosts_path"`
	ResolvConfPath    string      `json:"resolv_conf_path"`
	DNS               []string    `json:"dns"`
	ExtraHosts        []extraHost `json:"extra_hosts"`
	UseDefaultSandbox bool        `json:"use_default_sandbox"`
	UseExternalKey    bool        `json:"use_external_key"`
}

// endpointJoin represents the expected body of the "join endpoint" or "leave endpoint" http request messages
type endpointJoin struct {
	SandboxID string `json:"sandbox_id"`
}

// servicePublish represents the body of the "publish service" http request message
type servicePublish struct {
	Name         string                `json:"name"`
	Network      string                `json:"network_name"`
	ExposedPorts []types.TransportPort `json:"exposed_ports"`
	PortMapping  []types.PortBinding   `json:"port_mapping"`
}

// extraHost represents the extra host object
type extraHost struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}
