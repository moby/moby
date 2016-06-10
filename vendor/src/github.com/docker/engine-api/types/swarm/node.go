package swarm

// Node represents a node.
type Node struct {
	ID string
	Meta

	Spec          NodeSpec        `json:",omitempty"`
	Description   NodeDescription `json:",omitempty"`
	Status        NodeStatus      `json:",omitempty"`
	ManagerStatus *ManagerStatus  `json:",omitempty"`
}

// NodeSpec represents the spec of a node.
type NodeSpec struct {
	Annotations
	Role         NodeRole         `json:",omitempty"`
	Membership   NodeMembership   `json:",omitempty"`
	Availability NodeAvailability `json:",omitempty"`
}

const (
	// NodeRoleWorker WORKER
	NodeRoleWorker NodeRole = "WORKER"
	// NodeRoleManager MANAGER
	NodeRoleManager NodeRole = "MANAGER"
)

// NodeRole represents the role of a node.
type NodeRole string

const (
	// NodeMembershipPending PENDING
	NodeMembershipPending NodeMembership = "PENDING"
	// NodeMembershipAccepted ACCEPTED
	NodeMembershipAccepted NodeMembership = "ACCEPTED"
	// NodeMembershipRejected REJECTED
	NodeMembershipRejected NodeMembership = "REJECTED"
)

// NodeMembership represents the membership of a node.
type NodeMembership string

const (
	// NodeAvailabilityActive ACTIVE
	NodeAvailabilityActive NodeAvailability = "ACTIVE"
	// NodeAvailabilityPause PAUSE
	NodeAvailabilityPause NodeAvailability = "PAUSE"
	// NodeAvailabilityDrain DRAIN
	NodeAvailabilityDrain NodeAvailability = "DRAIN"
)

// NodeAvailability represents the availability of a node.
type NodeAvailability string

// NodeDescription represents the description of a node.
type NodeDescription struct {
	Hostname  string            `json:",omitempty"`
	Platform  Platform          `json:",omitempty"`
	Resources Resources         `json:",omitempty"`
	Engine    EngineDescription `json:",omitempty"`
}

// Platform represents the platfrom (Arch/OS).
type Platform struct {
	Architecture string `json:",omitempty"`
	OS           string `json:",omitempty"`
}

// EngineDescription represents the description of an engine.
type EngineDescription struct {
	EngineVersion string              `json:",omitempty"`
	Labels        map[string]string   `json:",omitempty"`
	Plugins       []PluginDescription `json:",omitempty"`
}

// PluginDescription represents the description of an engine plugin.
type PluginDescription struct {
	Type string `json:",omitempty"`
	Name string `json:",omitempty"`
}

// NodeStatus represents the status of a node.
type NodeStatus struct {
	State   NodeState `json:",omitempty"`
	Message string    `json:",omitempty"`
}

const (
	// ReachabilityUnknown UNKNOWN
	ReachabilityUnknown Reachability = "UNKNOWN"
	// ReachabilityUnreachable UNREACHABLE
	ReachabilityUnreachable Reachability = "UNREACHABLE"
	// ReachabilityReachable REACHABLE
	ReachabilityReachable Reachability = "REACHABLE"
)

// Reachability represents the reachability of a node.
type Reachability string

// ManagerStatus represents the status of a manager.
type ManagerStatus struct {
	Leader       bool         `json:",omitempty"`
	Reachability Reachability `json:",omitempty"`
	Message      string       `json:",omitempty"`
	Addr         string       `json:",omitempty"`
}

const (
	// NodeStateUnknown UNKNOWN
	NodeStateUnknown NodeState = "UNKNOWN"
	// NodeStateDown DOWN
	NodeStateDown NodeState = "DOWN"
	// NodeStateReady READY
	NodeStateReady NodeState = "READY"
	// NodeStateDisconnected DISCONNECTED
	NodeStateDisconnected NodeState = "DISCONNECTED"
)

// NodeState represents the state of a node.
type NodeState string
