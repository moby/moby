package scope

// Data scopes.
const (
	// Local indicates to store the KV object in local datastore such as boltdb
	Local = "local"
	// Global indicates to store the KV object in global datastore
	Global = "global"
	// Swarm is not indicating a datastore location. It is defined here
	// along with the other two scopes just for consistency.
	Swarm = "swarm"
)
