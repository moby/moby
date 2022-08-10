package membership

import (
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/watch"
	"go.etcd.io/etcd/raft/v3/raftpb"
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
	// ErrMemberRemoved is thrown when a node was removed from the cluster
	ErrMemberRemoved = errors.New("raft: member was removed from the cluster")
)

// Cluster represents a set of active
// raft Members
type Cluster struct {
	mu      sync.RWMutex
	members map[uint64]*Member

	// removed contains the list of removed Members,
	// those ids cannot be reused
	removed map[uint64]bool

	PeersBroadcast *watch.Queue
}

// Member represents a raft Cluster Member
type Member struct {
	*api.RaftMember
}

// NewCluster creates a new Cluster neighbors list for a raft Member.
func NewCluster() *Cluster {
	// TODO(abronan): generate Cluster ID for federation

	return &Cluster{
		members:        make(map[uint64]*Member),
		removed:        make(map[uint64]bool),
		PeersBroadcast: watch.NewQueue(),
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

func (c *Cluster) broadcastUpdate() {
	peers := make([]*api.Peer, 0, len(c.members))
	for _, m := range c.members {
		peers = append(peers, &api.Peer{
			NodeID: m.NodeID,
			Addr:   m.Addr,
		})
	}
	c.PeersBroadcast.Publish(peers)
}

// AddMember adds a node to the Cluster Memberlist.
func (c *Cluster) AddMember(member *Member) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.removed[member.RaftID] {
		return ErrIDRemoved
	}

	c.members[member.RaftID] = member

	c.broadcastUpdate()
	return nil
}

// RemoveMember removes a node from the Cluster Memberlist, and adds it to
// the removed list.
func (c *Cluster) RemoveMember(id uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removed[id] = true

	return c.clearMember(id)
}

// UpdateMember updates member address.
func (c *Cluster) UpdateMember(id uint64, m *api.RaftMember) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.removed[id] {
		return ErrIDRemoved
	}

	oldMember, ok := c.members[id]
	if !ok {
		return ErrIDNotFound
	}

	if oldMember.NodeID != m.NodeID {
		// Should never happen; this is a sanity check
		return errors.New("node ID mismatch match on node update")
	}

	if oldMember.Addr == m.Addr {
		// nothing to do
		return nil
	}
	oldMember.RaftMember = m
	c.broadcastUpdate()
	return nil
}

// ClearMember removes a node from the Cluster Memberlist, but does NOT add it
// to the removed list.
func (c *Cluster) ClearMember(id uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.clearMember(id)
}

func (c *Cluster) clearMember(id uint64) error {
	if _, ok := c.members[id]; ok {
		delete(c.members, id)
		c.broadcastUpdate()
	}
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
