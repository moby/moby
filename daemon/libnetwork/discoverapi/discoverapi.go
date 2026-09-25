package discoverapi

// Discover is an interface to be implemented by the component interested in receiving discover events
// like new node joining the cluster or datastore updates
type Discover interface {
	// DiscoverNew is a notification for a new discovery event, Example:a new node joining a cluster
	DiscoverNew(dType DiscoveryType, data any) error

	// DiscoverDelete is a notification for a discovery delete event, Example:a node leaving a cluster
	DiscoverDelete(dType DiscoveryType, data any) error
}

// DiscoveryType represents the type of discovery element the DiscoverNew function is invoked on
type DiscoveryType int

const (
	NodeDiscovery        DiscoveryType = 1 // NodeDiscovery represents Node join/leave events provided by discovery.
	EncryptionKeysConfig DiscoveryType = 2 // EncryptionKeysConfig represents the initial key(s) for performing datapath encryption.
	EncryptionKeysUpdate DiscoveryType = 3 // EncryptionKeysUpdate represents an update to the datapath encryption key(s).
)

// NodeDiscoveryData represents the structure backing the node discovery data json string
type NodeDiscoveryData struct {
	Address     string
	BindAddress string
	Self        bool
}

// DriverEncryptionConfig contains the initial datapath encryption key(s)
// Key in first position is the primary key, the one to be used in tx.
// Original key and tag types are []byte and uint64
type DriverEncryptionConfig struct {
	Keys [][]byte
	Tags []uint64
}

// DriverEncryptionUpdate carries an update to the encryption key(s) as:
// a new key and/or set a primary key and/or a removal of an existing key.
// Original key and tag types are []byte and uint64
type DriverEncryptionUpdate struct {
	Key        []byte
	Tag        uint64
	Primary    []byte
	PrimaryTag uint64
	Prune      []byte
	PruneTag   uint64
}
