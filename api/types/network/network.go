package network // import "github.com/docker/docker/api/types/network"
import (
	"github.com/docker/docker/api/types/filters"
)

// Address represents an IP address
type Address struct {
	Addr      string
	PrefixLen int
}

// PeerInfo represents one peer of an overlay network
type PeerInfo struct {
	Name string
	IP   string
}

// Task carries the information about one backend task
type Task struct {
	Name       string
	EndpointID string
	EndpointIP string
	Info       map[string]string
}

// ServiceInfo represents service parameters with the list of service's tasks
type ServiceInfo struct {
	VIP          string
	Ports        []string
	LocalLBIndex int
	Tasks        []Task
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings // Endpoint configs for each connecting network
}

// ConfigReference specifies the source which provides a network's configuration
type ConfigReference struct {
	Network string
}

var acceptedFilters = map[string]bool{
	"dangling": true,
	"driver":   true,
	"id":       true,
	"label":    true,
	"name":     true,
	"scope":    true,
	"type":     true,
}

// ValidateFilters validates the list of filter args with the available filters.
func ValidateFilters(filter filters.Args) error {
	return filter.Validate(acceptedFilters)
}
