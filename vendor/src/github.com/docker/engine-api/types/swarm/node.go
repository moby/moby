package swarm

// Node represents a node.
type Node struct {
	ID string
	Meta

	Spec        NodeSpec        `json:",omitempty"`
	Description NodeDescription `json:",omitempty"`
	Status      NodeStatus      `json:",omitempty"`
	// TODO(aluzzardi): This should probably be ManagerStatus.
	Manager *Manager `json:",omitempty"`
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
	NodeRoleWorker = "WORKER"
	// NodeRoleManager MANAGER
	NodeRoleManager = "MANAGER"
)

// NodeRole represents the role of a node.
type NodeRole string

const (
	// NodeMembershipPending PENDING
	NodeMembershipPending = "PENDING"
	// NodeMembershipAccepted ACCEPTED
	NodeMembershipAccepted = "ACCEPTED"
	// NodeMembershipRejected REJECTED
	NodeMembershipRejected = "REJECTED"
)

// NodeMembership represents the membership of a node.
type NodeMembership string

const (
	// NodeAvailabilityActive ACTIVE
	NodeAvailabilityActive = "ACTIVE"
	// NodeAvailabilityPause PAUSE
	NodeAvailabilityPause = "PAUSE"
	// NodeAvailabilityDrain DRAIN
	NodeAvailabilityDrain = "DRAIN"
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
	ReachabilityUnknown = "UNKNOWN"
	// ReachabilityUnreachable UNREACHABLE
	ReachabilityUnreachable = "UNREACHABLE"
	// ReachabilityReachable REACHABLE
	ReachabilityReachable = "REACHABLE"
)

// Reachability represents the reachability of a node.
type Reachability string

// RaftMemberStatus represents the raft status of a raft member.
type RaftMemberStatus struct {
	Leader       bool         `json:",omitempty"`
	Reachability Reachability `json:",omitempty"`
	Message      string       `json:",omitempty"`
}

// RaftMember represents a raft member.
type RaftMember struct {
	RaftID uint64           `json:",omitempty"`
	Addr   string           `json:",omitempty"`
	Status RaftMemberStatus `json:",omitempty"`
}

// Manager represents a manager.
type Manager struct {
	Raft RaftMember `json:",omitempty"`
}

const (
	// NodeStateUnknown UNKNOWN
	NodeStateUnknown = "UNKNOWN"
	// NodeStateDown DOWN
	NodeStateDown = "DOWN"
	// NodeStateReady READY
	NodeStateReady = "READY"
	// NodeStateDisconnected DISCONNECTED
	NodeStateDisconnected = "DISCONNECTED"
)

// NodeState represents the state of a node.
type NodeState string
