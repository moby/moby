// Code generated from OpenAPI definition. DO NOT EDIT.

package network

import (
	"net/netip"
)

// Task carries the information about one backend task
type Task struct {
	//
	Name string `json:"Name,omitempty"`

	//
	EndpointID string `json:"EndpointID,omitempty"`

	//
	EndpointIP netip.Addr `json:"EndpointIP,omitempty"`

	//
	Info map[string]string `json:"Info,omitempty"`
}
