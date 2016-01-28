package discoverapi

// Discover is an interface to be implemented by the componenet interested in receiving discover events
// like new node joining the cluster or datastore updates
type Discover interface {
	// DiscoverNew is a notification for a new discovery event, Example:a new node joining a cluster
	DiscoverNew(dType DiscoveryType, data interface{}) error

	// DiscoverDelete is a notification for a discovery delete event, Example:a node leaving a cluster
	DiscoverDelete(dType DiscoveryType, data interface{}) error
}

// DiscoveryType represents the type of discovery element the DiscoverNew function is invoked on
type DiscoveryType int

const (
	// NodeDiscovery represents Node join/leave events provided by discovery
	NodeDiscovery = iota + 1
	// DatastoreConfig represents a add/remove datastore event
	DatastoreConfig
)

// NodeDiscoveryData represents the structure backing the node discovery data json string
type NodeDiscoveryData struct {
	Address string
	Self    bool
}

// DatastoreConfigData is the data for the datastore update event message
type DatastoreConfigData struct {
	Scope    string
	Provider string
	Address  string
	Config   interface{}
}
