package client

import "github.com/docker/libnetwork/sandbox"

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
	Info    sandbox.Info
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
