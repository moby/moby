package swarm

import "time"

// Swarm represents s swarm.
type Swarm struct {
	ID string
	Meta
	Spec Spec
}

// Spec represents the spec of a swarm.
type Spec struct {
	Annotations

	AcceptancePolicy AcceptancePolicy    `json:",omitempty"`
	Orchestration    OrchestrationConfig `json:",omitempty"`
	Raft             RaftConfig          `json:",omitempty"`
	Dispatcher       DispatcherConfig    `json:",omitempty"`
	CAConfig         CAConfig            `json:",omitempty"`
}

// AcceptancePolicy represents the list of policies.
type AcceptancePolicy struct {
	Policies []Policy `json:",omitempty"`
}

// Policy represents a role, autoaccept and secret.
type Policy struct {
	Role       string
	Autoaccept bool
	Secret     string `json:",omitempty"`
}

// OrchestrationConfig represents ochestration configuration.
type OrchestrationConfig struct {
	TaskHistoryRetentionLimit int64 `json:",omitempty"`
}

// RaftConfig represents raft configuration.
type RaftConfig struct {
	SnapshotInterval           uint64 `json:",omitempty"`
	KeepOldSnapshots           uint64 `json:",omitempty"`
	LogEntriesForSlowFollowers uint64 `json:",omitempty"`
	HeartbeatTick              uint32 `json:",omitempty"`
	ElectionTick               uint32 `json:",omitempty"`
}

// DispatcherConfig represents dispatcher configuration.
type DispatcherConfig struct {
	HeartbeatPeriod uint64 `json:",omitempty"`
}

// CAConfig represents CA configuration.
type CAConfig struct {
	NodeCertExpiry time.Duration `json:",omitempty"`
}

// InitRequest is the request used to init a swarm.
type InitRequest struct {
	ListenAddr      string
	ForceNewCluster bool
	Spec            Spec
}

// JoinRequest is the request used to join a swarm.
type JoinRequest struct {
	ListenAddr string
	RemoteAddr string
	Secret     string // accept by secret
	CAHash     string
	Manager    bool
}

// Info represents generic information about swarm.
// TODO(aluzzardi) We should provide more status information about Swarm.
type Info struct {
	NodeID string
	// TODO(aluzzardi): This should be a NodeRole.
	IsAgent   bool
	IsManager bool
	// TODO(aluzzardi): Should we export this? At least we should rename it
	// (this is the list of managers the agent is connected to).
	Remotes  map[string]string
	Nodes    int
	Managers int
}
