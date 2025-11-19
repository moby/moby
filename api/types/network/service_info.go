// Code generated from OpenAPI definition. DO NOT EDIT.

package network

import (
	"net/netip"
)

// ServiceInfo represents service parameters with the list of service's tasks
type ServiceInfo struct {
	//
	VIP netip.Addr `json:"VIP,omitempty"`

	//
	Ports []string `json:"Ports,omitempty"`

	//
	LocalLBIndex int `json:"LocalLBIndex,omitempty"`

	//
	Tasks []NetworkTaskInfo `json:"Tasks,omitempty"`
}
