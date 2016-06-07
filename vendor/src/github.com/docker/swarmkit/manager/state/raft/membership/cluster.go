package membership

import (
	"errors"
	"sync"

	"google.golang.org/grpc"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/docker/swarmkit/api"
	"github.com/gogo/protobuf/proto"
)

var (
	// ErrIDExists is thrown when a node wants to join the existing cluster but its ID already exists
	ErrIDExists = errors.New("membership: can't add node to cluster, node id is a duplicate")
	// ErrIDRemoved is thrown when a node tries to perform an operation on an existing cluster but was removed
	ErrIDRemoved = errors.New("membership: node was removed during cluster lifetime")
	// ErrIDNotFound is thrown when we try an operation on a member that does not exist in the cluster list
	ErrIDNotFound = errors.New("membership: member not found in cluster list")
	// ErrConfigChangeInvalid is thrown when a configuration change we received looks invalid in form
	ErrConfigChangeInvalid = errors.New("membership: ConfChange type should be either AddNode, RemoveNode or UpdateNode")
	// ErrCannotUnmarshalConfig is thrown when a node cannot unmarshal a configuration change
	ErrCannotUnmarshalConfig = errors.New("membership: cannot unmarshal configuration change")
)

// Cluster represents a set of active
// raft Members
type Cluster struct {
	id uint64

	mu      sync.RWMutex
	members map[uint64]*Member

	// removed contains the list of removed Members,
	// those ids cannot be reused
	removed map[uint64]bool
}

// Member represents a raft Cluster Member
type Member struct {
	*api.RaftMember

	api.RaftClient
	Conn *grpc.ClientConn
}

// NewCluster creates a new Cluster neighbors
// list for a raft Member
func NewCluster() *Cluster {
	// TODO(abronan): generate Cluster ID for federation

	return &Cluster{
		members: make(map[uint64]*Member),
		removed: make(map[uint64]bool),
	}
}

// Members returns the list of raft Members in the Cluster.
func (c *Cluster) Members() map[uint64]*Member {
	members := make(map[uint64]*Member)
	c.mu.RLock()
	for k, v := range c.members {
		members[k] = v
	}
	c.mu.RUnlock()
	return members
}

// Removed returns the list of raft Members removed from the Cluster.
func (c *Cluster) Removed() []uint64 {
	c.mu.RLock()
	removed := make([]uint64, 0, len(c.removed))
	for k := range c.removed {
		removed = append(removed, k)
	}
	c.mu.RUnlock()
	return removed
}

// GetMember returns informations on a given Member.
func (c *Cluster) GetMember(id uint64) *Member {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.members[id]
}

// AddMember adds a node to the Cluster Memberlist.
func (c *Cluster) AddMember(member *Member) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.removed[member.RaftID] {
		return ErrIDRemoved
	}

	c.members[member.RaftID] = member
	return nil
}

// RemoveMember removes a node from the Cluster Memberlist.
func (c *Cluster) RemoveMember(id uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.members[id] == nil {
		return ErrIDNotFound
	}

	conn := c.members[id].Conn
	if conn != nil {
		_ = conn.Close()
	}

	c.removed[id] = true
	delete(c.members, id)
	return nil
}

// IsIDRemoved checks if a Member is in the remove set.
func (c *Cluster) IsIDRemoved(id uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.removed[id]
}

// Clear resets the list of active Members and removed Members.
func (c *Cluster) Clear() {
	c.mu.Lock()
	c.members = make(map[uint64]*Member)
	c.removed = make(map[uint64]bool)
	c.mu.Unlock()
}

// ValidateConfigurationChange takes a proposed ConfChange and
// ensures that it is valid.
func (c *Cluster) ValidateConfigurationChange(cc raftpb.ConfChange) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.removed[cc.NodeID] {
		return ErrIDRemoved
	}
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		if c.members[cc.NodeID] != nil {
			return ErrIDExists
		}
	case raftpb.ConfChangeRemoveNode:
		if c.members[cc.NodeID] == nil {
			return ErrIDNotFound
		}
	case raftpb.ConfChangeUpdateNode:
		if c.members[cc.NodeID] == nil {
			return ErrIDNotFound
		}
	default:
		return ErrConfigChangeInvalid
	}
	m := &api.RaftMember{}
	if err := proto.Unmarshal(cc.Context, m); err != nil {
		return ErrCannotUnmarshalConfig
	}
	return nil
}

// CanRemoveMember checks if removing a Member would not result in a loss
// of quorum, this check is needed before submitting a configuration change
// that might block or harm the Cluster on Member recovery
func (c *Cluster) CanRemoveMember(from uint64, id uint64) bool {
	members := c.Members()

	nmembers := 0
	nreachable := 0

	for _, m := range members {
		// Skip the node that is going to be deleted
		if m.RaftID == id {
			continue
		}

		// Local node from where the remove is issued
		if m.RaftID == from {
			nmembers++
			nreachable++
			continue
		}

		connState, err := m.Conn.State()
		if err == nil && connState == grpc.Ready {
			nreachable++
		}

		nmembers++
	}

	// Special case of 2 managers
	if nreachable == 1 && len(members) <= 2 {
		return false
	}

	nquorum := nmembers/2 + 1
	if nreachable < nquorum {
		return false
	}

	return true
}
