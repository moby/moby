package raft

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/pkg/idutil"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/raft/membership"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/clock"
)

var (
	// ErrHealthCheckFailure is returned when there is an issue with the initial handshake which means
	// that the address provided must be invalid or there is ongoing connectivity issues at join time.
	ErrHealthCheckFailure = errors.New("raft: could not connect to prospective new cluster member using its advertised address")
	// ErrNoRaftMember is thrown when the node is not yet part of a raft cluster
	ErrNoRaftMember = errors.New("raft: node is not yet part of a raft cluster")
	// ErrConfChangeRefused is returned when there is an issue with the configuration change
	ErrConfChangeRefused = errors.New("raft: propose configuration change refused")
	// ErrApplyNotSpecified is returned during the creation of a raft node when no apply method was provided
	ErrApplyNotSpecified = errors.New("raft: apply method was not specified")
	// ErrAppendEntry is thrown when the node fail to append an entry to the logs
	ErrAppendEntry = errors.New("raft: failed to append entry to logs")
	// ErrSetHardState is returned when the node fails to set the hard state
	ErrSetHardState = errors.New("raft: failed to set the hard state for log append entry")
	// ErrApplySnapshot is returned when the node fails to apply a snapshot
	ErrApplySnapshot = errors.New("raft: failed to apply snapshot on raft node")
	// ErrStopped is returned when an operation was submitted but the node was stopped in the meantime
	ErrStopped = errors.New("raft: failed to process the request: node is stopped")
	// ErrLostLeadership is returned when an operation was submitted but the node lost leader status before it became committed
	ErrLostLeadership = errors.New("raft: failed to process the request: node lost leader status")
	// ErrRequestTooLarge is returned when a raft internal message is too large to be sent
	ErrRequestTooLarge = errors.New("raft: raft message is too large and can't be sent")
	// ErrCannotRemoveMember is thrown when we try to remove a member from the cluster but this would result in a loss of quorum
	ErrCannotRemoveMember = errors.New("raft: member cannot be removed, because removing it may result in loss of quorum")
	// ErrMemberRemoved is thrown when a node was removed from the cluster
	ErrMemberRemoved = errors.New("raft: member was removed from the cluster")
	// ErrNoClusterLeader is thrown when the cluster has no elected leader
	ErrNoClusterLeader = errors.New("raft: no elected cluster leader")
)

// LeadershipState indicates whether the node is a leader or follower.
type LeadershipState int

const (
	// IsLeader indicates that the node is a raft leader.
	IsLeader LeadershipState = iota
	// IsFollower indicates that the node is a raft follower.
	IsFollower
)

// Node represents the Raft Node useful
// configuration.
type Node struct {
	raft.Node
	cluster *membership.Cluster

	Server         *grpc.Server
	Ctx            context.Context
	cancel         func()
	tlsCredentials credentials.TransportAuthenticator

	Address  string
	StateDir string
	Error    error

	raftStore   *raft.MemoryStorage
	memoryStore *store.MemoryStore
	Config      *raft.Config
	opts        NewNodeOptions
	reqIDGen    *idutil.Generator
	wait        *wait
	wal         *wal.WAL
	snapshotter *snap.Snapshotter
	wasLeader   bool
	restored    bool
	isMember    uint32
	joinAddr    string

	// waitProp waits for all the proposals to be terminated before
	// shutting down the node.
	waitProp sync.WaitGroup

	// forceNewCluster is a special flag used to recover from disaster
	// scenario by pointing to an existing or backed up data directory.
	forceNewCluster bool

	confState     raftpb.ConfState
	appliedIndex  uint64
	snapshotIndex uint64

	ticker      clock.Ticker
	sendTimeout time.Duration
	stopCh      chan struct{}
	doneCh      chan struct{}
	// removeRaftCh notifies about node deletion from raft cluster
	removeRaftCh        chan struct{}
	removeRaftOnce      sync.Once
	leadershipBroadcast *events.Broadcaster

	// used to coordinate shutdown
	stopMu sync.RWMutex
	// used for membership management checks
	membershipLock sync.Mutex

	snapshotInProgress chan uint64
	asyncTasks         sync.WaitGroup
}

// NewNodeOptions provides arguments for NewNode
type NewNodeOptions struct {
	// ID is the node's ID, from its certificate's CN field.
	ID string
	// Addr is the address of this node's listener
	Addr string
	// ForceNewCluster defines if we have to force a new cluster
	// because we are recovering from a backup data directory.
	ForceNewCluster bool
	// JoinAddr is the cluster to join. May be an empty string to create
	// a standalone cluster.
	JoinAddr string
	// Config is the raft config.
	Config *raft.Config
	// StateDir is the directory to store durable state.
	StateDir string
	// TickInterval interval is the time interval between raft ticks.
	TickInterval time.Duration
	// ClockSource is a Clock interface to use as a time base.
	// Leave this nil except for tests that are designed not to run in real
	// time.
	ClockSource clock.Clock
	// SendTimeout is the timeout on the sending messages to other raft
	// nodes. Leave this as 0 to get the default value.
	SendTimeout    time.Duration
	TLSCredentials credentials.TransportAuthenticator
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// NewNode generates a new Raft node
func NewNode(ctx context.Context, opts NewNodeOptions) *Node {
	cfg := opts.Config
	if cfg == nil {
		cfg = DefaultNodeConfig()
	}
	if opts.TickInterval == 0 {
		opts.TickInterval = time.Second
	}

	raftStore := raft.NewMemoryStorage()

	ctx, cancel := context.WithCancel(ctx)

	n := &Node{
		Ctx:            ctx,
		cancel:         cancel,
		cluster:        membership.NewCluster(),
		tlsCredentials: opts.TLSCredentials,
		raftStore:      raftStore,
		Address:        opts.Addr,
		opts:           opts,
		Config: &raft.Config{
			ElectionTick:    cfg.ElectionTick,
			HeartbeatTick:   cfg.HeartbeatTick,
			Storage:         raftStore,
			MaxSizePerMsg:   cfg.MaxSizePerMsg,
			MaxInflightMsgs: cfg.MaxInflightMsgs,
			Logger:          cfg.Logger,
		},
		forceNewCluster:     opts.ForceNewCluster,
		stopCh:              make(chan struct{}),
		doneCh:              make(chan struct{}),
		removeRaftCh:        make(chan struct{}),
		StateDir:            opts.StateDir,
		joinAddr:            opts.JoinAddr,
		sendTimeout:         2 * time.Second,
		leadershipBroadcast: events.NewBroadcaster(),
	}
	n.memoryStore = store.NewMemoryStore(n)

	if opts.ClockSource == nil {
		n.ticker = clock.NewClock().NewTicker(opts.TickInterval)
	} else {
		n.ticker = opts.ClockSource.NewTicker(opts.TickInterval)
	}
	if opts.SendTimeout != 0 {
		n.sendTimeout = opts.SendTimeout
	}

	n.reqIDGen = idutil.NewGenerator(uint16(n.Config.ID), time.Now())
	n.wait = newWait()

	return n
}

// JoinAndStart joins and starts the raft server
func (n *Node) JoinAndStart() error {
	loadAndStartErr := n.loadAndStart(n.Ctx, n.opts.ForceNewCluster)
	if loadAndStartErr != nil && loadAndStartErr != errNoWAL {
		n.ticker.Stop()
		return loadAndStartErr
	}

	snapshot, err := n.raftStore.Snapshot()
	// Snapshot never returns an error
	if err != nil {
		panic("could not get snapshot of raft store")
	}

	n.confState = snapshot.Metadata.ConfState
	n.appliedIndex = snapshot.Metadata.Index
	n.snapshotIndex = snapshot.Metadata.Index

	if loadAndStartErr == errNoWAL {
		if n.joinAddr != "" {
			c, err := n.ConnectToMember(n.joinAddr, 10*time.Second)
			if err != nil {
				return err
			}
			client := api.NewRaftMembershipClient(c.Conn)
			defer func() {
				_ = c.Conn.Close()
			}()

			ctx, cancel := context.WithTimeout(n.Ctx, 10*time.Second)
			defer cancel()
			resp, err := client.Join(ctx, &api.JoinRequest{
				Addr: n.Address,
			})
			if err != nil {
				return err
			}

			n.Config.ID = resp.RaftID

			if _, err := n.createWAL(n.opts.ID); err != nil {
				return err
			}

			n.Node = raft.StartNode(n.Config, []raft.Peer{})

			if err := n.registerNodes(resp.Members); err != nil {
				return err
			}
		} else {
			// First member in the cluster, self-assign ID
			n.Config.ID = uint64(rand.Int63()) + 1
			peer, err := n.createWAL(n.opts.ID)
			if err != nil {
				return err
			}
			n.Node = raft.StartNode(n.Config, []raft.Peer{peer})
			if err := n.Campaign(n.Ctx); err != nil {
				return err
			}
		}
		atomic.StoreUint32(&n.isMember, 1)
		return nil
	}

	if n.joinAddr != "" {
		n.Config.Logger.Warning("ignoring request to join cluster, because raft state already exists")
	}
	n.Node = raft.RestartNode(n.Config)
	atomic.StoreUint32(&n.isMember, 1)
	return nil
}

// DefaultNodeConfig returns the default config for a
// raft node that can be modified and customized
func DefaultNodeConfig() *raft.Config {
	return &raft.Config{
		HeartbeatTick:   1,
		ElectionTick:    3,
		MaxSizePerMsg:   math.MaxUint16,
		MaxInflightMsgs: 256,
		Logger:          log.L,
	}
}

// DefaultRaftConfig returns a default api.RaftConfig.
func DefaultRaftConfig() api.RaftConfig {
	return api.RaftConfig{
		KeepOldSnapshots:           0,
		SnapshotInterval:           10000,
		LogEntriesForSlowFollowers: 500,
		ElectionTick:               3,
		HeartbeatTick:              1,
	}
}

// MemoryStore returns the memory store that is kept in sync with the raft log.
func (n *Node) MemoryStore() *store.MemoryStore {
	return n.memoryStore
}

// Run is the main loop for a Raft node, it goes along the state machine,
// acting on the messages received from other Raft nodes in the cluster.
//
// Before running the main loop, it first starts the raft node based on saved
// cluster state. If no saved state exists, it starts a single-node cluster.
func (n *Node) Run(ctx context.Context) error {
	defer func() {
		close(n.doneCh)
	}()

	for {
		select {
		case <-n.ticker.C():
			n.Tick()

		case rd := <-n.Ready():
			raftConfig := DefaultRaftConfig()
			n.memoryStore.View(func(readTx store.ReadTx) {
				clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
				if err == nil && len(clusters) == 1 {
					raftConfig = clusters[0].Spec.Raft
				}
			})

			// Save entries to storage
			if err := n.saveToStorage(&raftConfig, rd.HardState, rd.Entries, rd.Snapshot); err != nil {
				n.Config.Logger.Error(err)
			}

			// Send raft messages to peers
			if err := n.send(rd.Messages); err != nil {
				n.Config.Logger.Error(err)
			}

			// Apply snapshot to memory store. The snapshot
			// was applied to the raft store in
			// saveToStorage.
			if !raft.IsEmptySnap(rd.Snapshot) {
				// Load the snapshot data into the store
				if err := n.restoreFromSnapshot(rd.Snapshot.Data, n.forceNewCluster); err != nil {
					n.Config.Logger.Error(err)
				}
				n.appliedIndex = rd.Snapshot.Metadata.Index
				n.snapshotIndex = rd.Snapshot.Metadata.Index
				n.confState = rd.Snapshot.Metadata.ConfState
			}

			// Process committed entries
			for _, entry := range rd.CommittedEntries {
				if err := n.processCommitted(entry); err != nil {
					n.Config.Logger.Error(err)
				}
			}

			// Trigger a snapshot every once in awhile
			if n.snapshotInProgress == nil &&
				raftConfig.SnapshotInterval > 0 &&
				n.appliedIndex-n.snapshotIndex >= raftConfig.SnapshotInterval {
				n.doSnapshot(&raftConfig)
			}

			// If we cease to be the leader, we must cancel
			// any proposals that are currently waiting for
			// a quorum to acknowledge them. It is still
			// possible for these to become committed, but
			// if that happens we will apply them as any
			// follower would.
			if rd.SoftState != nil {
				if n.wasLeader && rd.SoftState.RaftState != raft.StateLeader {
					n.wasLeader = false
					n.wait.cancelAll()
					n.leadershipBroadcast.Write(IsFollower)
				} else if !n.wasLeader && rd.SoftState.RaftState == raft.StateLeader {
					n.wasLeader = true
					n.leadershipBroadcast.Write(IsLeader)
				}
			}

			// If we are the only registered member after
			// restoring from the state, campaign to be the
			// leader.
			if !n.restored {
				// Node ID should be in the progress list to Campaign
				_, ok := n.Node.Status().Progress[n.Config.ID]
				if len(n.cluster.Members()) <= 1 && ok {
					if err := n.Campaign(n.Ctx); err != nil {
						panic("raft: cannot campaign to be the leader on node restore")
					}
				}
				n.restored = true
			}

			// Advance the state machine
			n.Advance()

		case snapshotIndex := <-n.snapshotInProgress:
			if snapshotIndex > n.snapshotIndex {
				n.snapshotIndex = snapshotIndex
			}
			n.snapshotInProgress = nil
		case <-n.removeRaftCh:
			// If the node was removed from other members,
			// send back an error to the caller to start
			// the shutdown process.
			n.stop()

			// Move WAL and snapshot out of the way, since
			// they are no longer usable.
			if err := n.moveWALAndSnap(); err != nil {
				n.Config.Logger.Error(err)
			}

			return ErrMemberRemoved
		case <-n.stopCh:
			n.stop()
			return nil
		}
	}
}

// Shutdown stops the raft node processing loop.
// Calling Shutdown on an already stopped node
// will result in a panic.
func (n *Node) Shutdown() {
	select {
	case <-n.doneCh:
	default:
		close(n.stopCh)
		<-n.doneCh
	}
}

func (n *Node) stop() {
	n.stopMu.Lock()
	defer n.stopMu.Unlock()

	n.cancel()
	n.waitProp.Wait()
	n.asyncTasks.Wait()

	members := n.cluster.Members()
	for _, member := range members {
		if member.Conn != nil {
			_ = member.Conn.Close()
		}
	}
	n.Stop()
	n.ticker.Stop()
	if err := n.wal.Close(); err != nil {
		n.Config.Logger.Errorf("raft: error closing WAL: %v", err)
	}
	// TODO(stevvooe): Handle ctx.Done()
}

// IsLeader checks if we are the leader or not
func (n *Node) IsLeader() bool {
	if !n.IsMember() {
		return false
	}

	if n.Node.Status().Lead == n.Config.ID {
		return true
	}
	return false
}

// Leader returns the id of the leader
func (n *Node) Leader() uint64 {
	if !n.IsMember() {
		return 0
	}
	return n.Node.Status().Lead
}

// Join asks to a member of the raft to propose
// a configuration change and add us as a member thus
// beginning the log replication process. This method
// is called from an aspiring member to an existing member
func (n *Node) Join(ctx context.Context, req *api.JoinRequest) (*api.JoinResponse, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	fields := logrus.Fields{
		"node.id": nodeInfo.NodeID,
		"method":  "(*Node).Join",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log := log.G(ctx).WithFields(fields)

	// can't stop the raft node while an async RPC is in progress
	n.stopMu.RLock()
	defer n.stopMu.RUnlock()

	n.membershipLock.Lock()
	defer n.membershipLock.Unlock()

	if !n.IsMember() {
		return nil, ErrNoRaftMember
	}

	if n.IsStopped() {
		log.WithError(ErrStopped).Errorf(ErrStopped.Error())
		return nil, ErrStopped
	}

	if !n.IsLeader() {
		return nil, ErrLostLeadership
	}

	// Find a unique ID for the joining member.
	var raftID uint64
	for {
		raftID = uint64(rand.Int63()) + 1
		if n.cluster.GetMember(raftID) == nil && !n.cluster.IsIDRemoved(raftID) {
			break
		}
	}

	remoteAddr := req.Addr

	// If the joining node sent an address like 0.0.0.0:4242, automatically
	// determine its actual address based on the GRPC connection. This
	// avoids the need for a prospective member to know its own address.

	requestHost, requestPort, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid address %s in raft join request", remoteAddr)
	}

	requestIP := net.ParseIP(requestHost)
	if requestIP != nil && requestIP.IsUnspecified() {
		remoteHost, _, err := net.SplitHostPort(nodeInfo.RemoteAddr)
		if err != nil {
			return nil, err
		}
		remoteAddr = net.JoinHostPort(remoteHost, requestPort)
	}

	// We do not bother submitting a configuration change for the
	// new member if we can't contact it back using its address
	if err := n.checkHealth(ctx, remoteAddr, 5*time.Second); err != nil {
		return nil, err
	}

	err = n.addMember(ctx, remoteAddr, raftID, nodeInfo.NodeID)
	if err != nil {
		log.WithError(err).Errorf("failed to add member")
		return nil, err
	}

	var nodes []*api.RaftMember
	for _, node := range n.cluster.Members() {
		nodes = append(nodes, &api.RaftMember{
			RaftID: node.RaftID,
			NodeID: node.NodeID,
			Addr:   node.Addr,
		})
	}
	log.Debugf("node joined")

	return &api.JoinResponse{Members: nodes, RaftID: raftID}, nil
}

// checkHealth tries to contact an aspiring member through its advertised address
// and checks if its raft server is running.
func (n *Node) checkHealth(ctx context.Context, addr string, timeout time.Duration) error {
	conn, err := dial(addr, "tcp", n.tlsCredentials, timeout)
	if err != nil {
		return err
	}

	client := api.NewHealthClient(conn)
	defer conn.Close()

	resp, err := client.Check(ctx, &api.HealthCheckRequest{Service: "Raft"})
	if err != nil {
		return ErrHealthCheckFailure
	}
	if resp != nil && resp.Status != api.HealthCheckResponse_SERVING {
		return ErrHealthCheckFailure
	}

	return nil
}

// addMember submits a configuration change to add a new member on the raft cluster.
func (n *Node) addMember(ctx context.Context, addr string, raftID uint64, nodeID string) error {
	node := api.RaftMember{
		RaftID: raftID,
		NodeID: nodeID,
		Addr:   addr,
	}

	meta, err := node.Marshal()
	if err != nil {
		return err
	}

	cc := raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  raftID,
		Context: meta,
	}

	// Wait for a raft round to process the configuration change
	err = n.configure(ctx, cc)
	return err
}

// Leave asks to a member of the raft to remove
// us from the raft cluster. This method is called
// from a member who is willing to leave its raft
// membership to an active member of the raft
func (n *Node) Leave(ctx context.Context, req *api.LeaveRequest) (*api.LeaveResponse, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	fields := logrus.Fields{
		"node.id": nodeInfo.NodeID,
		"method":  "(*Node).Leave",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log.G(ctx).WithFields(fields).Debugf("")

	// can't stop the raft node while an async RPC is in progress
	n.stopMu.RLock()
	defer n.stopMu.RUnlock()

	if !n.IsMember() {
		return nil, ErrNoRaftMember
	}

	if n.IsStopped() {
		return nil, ErrStopped
	}

	if !n.IsLeader() {
		return nil, ErrLostLeadership
	}

	err = n.RemoveMember(ctx, req.Node.RaftID)
	if err != nil {
		return nil, err
	}

	return &api.LeaveResponse{}, nil
}

// CanRemoveMember checks if a member can be removed from
// the context of the current node.
func (n *Node) CanRemoveMember(id uint64) bool {
	return n.cluster.CanRemoveMember(n.Config.ID, id)
}

// RemoveMember submits a configuration change to remove a member from the raft cluster
// after checking if the operation would not result in a loss of quorum.
func (n *Node) RemoveMember(ctx context.Context, id uint64) error {
	n.membershipLock.Lock()
	defer n.membershipLock.Unlock()

	if n.cluster.CanRemoveMember(n.Config.ID, id) {
		cc := raftpb.ConfChange{
			ID:      id,
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  id,
			Context: []byte(""),
		}

		err := n.configure(ctx, cc)
		return err
	}

	return ErrCannotRemoveMember
}

// ProcessRaftMessage calls 'Step' which advances the
// raft state machine with the provided message on the
// receiving node
func (n *Node) ProcessRaftMessage(ctx context.Context, msg *api.ProcessRaftMessageRequest) (*api.ProcessRaftMessageResponse, error) {
	// Don't process the message if this comes from
	// a node in the remove set
	if n.cluster.IsIDRemoved(msg.Message.From) {
		return nil, ErrMemberRemoved
	}

	// can't stop the raft node while an async RPC is in progress
	n.stopMu.RLock()
	defer n.stopMu.RUnlock()

	if !n.IsMember() {
		return nil, ErrNoRaftMember
	}

	if n.IsStopped() {
		return nil, ErrStopped
	}

	if err := n.Step(n.Ctx, *msg.Message); err != nil {
		return nil, err
	}

	return &api.ProcessRaftMessageResponse{}, nil
}

// ResolveAddress returns the address reaching for a given node ID.
func (n *Node) ResolveAddress(ctx context.Context, msg *api.ResolveAddressRequest) (*api.ResolveAddressResponse, error) {
	if !n.IsMember() {
		return nil, ErrNoRaftMember
	}

	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	fields := logrus.Fields{
		"node.id": nodeInfo.NodeID,
		"method":  "(*Node).ResolveAddress",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log.G(ctx).WithFields(fields).Debugf("")

	member := n.cluster.GetMember(msg.RaftID)
	if member == nil {
		return nil, grpc.Errorf(codes.NotFound, "member %x not found", msg.RaftID)
	}
	return &api.ResolveAddressResponse{Addr: member.Addr}, nil
}

// LeaderAddr returns address of current cluster leader.
// With this method Node satisfies raftpicker.AddrSelector interface.
func (n *Node) LeaderAddr() (string, error) {
	n.stopMu.RLock()
	defer n.stopMu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := WaitForLeader(ctx, n); err != nil {
		return "", ErrNoClusterLeader
	}
	if n.IsStopped() {
		return "", ErrStopped
	}
	ms := n.cluster.Members()
	l := ms[n.Leader()]
	if l == nil {
		return "", ErrNoClusterLeader
	}
	return l.Addr, nil
}

// registerNode registers a new node on the cluster memberlist
func (n *Node) registerNode(node *api.RaftMember) error {
	if n.cluster.IsIDRemoved(node.RaftID) {
		return nil
	}

	member := &membership.Member{}

	existingMember := n.cluster.GetMember(node.RaftID)
	if existingMember != nil {
		// Member already exists

		// If the address is different from what we thought it was,
		// update it. This can happen if we just joined a cluster
		// and are adding ourself now with the remotely-reachable
		// address.
		if existingMember.Addr != node.Addr {
			member.RaftMember = node
			member.RaftClient = existingMember.RaftClient
			member.Conn = existingMember.Conn
			n.cluster.AddMember(member)
		}

		return nil
	}

	// Avoid opening a connection to the local node
	if node.RaftID != n.Config.ID {
		// We don't want to impose a timeout on the grpc connection. It
		// should keep retrying as long as necessary, in case the peer
		// is temporarily unavailable.
		var err error
		if member, err = n.ConnectToMember(node.Addr, 0); err != nil {
			return err
		}
	}

	member.RaftMember = node
	err := n.cluster.AddMember(member)
	if err != nil {
		if member.Conn != nil {
			_ = member.Conn.Close()
		}
		return err
	}
	return nil
}

// registerNodes registers a set of nodes in the cluster
func (n *Node) registerNodes(nodes []*api.RaftMember) error {
	for _, node := range nodes {
		if err := n.registerNode(node); err != nil {
			return err
		}
	}

	return nil
}

// ProposeValue calls Propose on the raft and waits
// on the commit log action before returning a result
func (n *Node) ProposeValue(ctx context.Context, storeAction []*api.StoreAction, cb func()) error {
	_, err := n.processInternalRaftRequest(ctx, &api.InternalRaftRequest{Action: storeAction}, cb)
	if err != nil {
		return err
	}
	return nil
}

// GetVersion returns the sequence information for the current raft round.
func (n *Node) GetVersion() *api.Version {
	status := n.Node.Status()
	return &api.Version{Index: status.Commit}
}

// GetMemberlist returns the current list of raft members in the cluster.
func (n *Node) GetMemberlist() map[uint64]*api.RaftMember {
	memberlist := make(map[uint64]*api.RaftMember)
	members := n.cluster.Members()
	leaderID := n.Leader()

	for id, member := range members {
		reachability := api.RaftMemberStatus_REACHABLE
		leader := false

		if member.RaftID != n.Config.ID {
			connState, err := member.Conn.State()
			if err != nil || connState != grpc.Ready {
				reachability = api.RaftMemberStatus_UNREACHABLE
			}
		}

		if member.RaftID == leaderID {
			leader = true
		}

		memberlist[id] = &api.RaftMember{
			RaftID: member.RaftID,
			NodeID: member.NodeID,
			Addr:   member.Addr,
			Status: api.RaftMemberStatus{
				Leader:       leader,
				Reachability: reachability,
			},
		}
	}

	return memberlist
}

// GetMemberByNodeID returns member information based
// on its generic Node ID.
func (n *Node) GetMemberByNodeID(nodeID string) *membership.Member {
	members := n.cluster.Members()
	for _, member := range members {
		if member.NodeID == nodeID {
			return member
		}
	}
	return nil
}

// IsMember checks if the raft node has effectively joined
// a cluster of existing members.
func (n *Node) IsMember() bool {
	return atomic.LoadUint32(&n.isMember) == 1
}

// IsStopped checks if the raft node is stopped or not
func (n *Node) IsStopped() bool {
	if n.Node == nil {
		return true
	}
	return false
}

// canSubmitProposal defines if any more proposals
// could be submitted and processed.
func (n *Node) canSubmitProposal() bool {
	select {
	case <-n.Ctx.Done():
		return false
	default:
		return true
	}
}

// Saves a log entry to our Store
func (n *Node) saveToStorage(raftConfig *api.RaftConfig, hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) (err error) {
	if !raft.IsEmptySnap(snapshot) {
		if err := n.saveSnapshot(snapshot, raftConfig.KeepOldSnapshots); err != nil {
			return ErrApplySnapshot
		}
		if err = n.raftStore.ApplySnapshot(snapshot); err != nil {
			return ErrApplySnapshot
		}
	}

	if err := n.wal.Save(hardState, entries); err != nil {
		// TODO(aaronl): These error types should really wrap more
		// detailed errors.
		return ErrApplySnapshot
	}

	if err = n.raftStore.Append(entries); err != nil {
		return ErrAppendEntry
	}

	return nil
}

// Sends a series of messages to members in the raft
func (n *Node) send(messages []raftpb.Message) error {
	members := n.cluster.Members()

	n.stopMu.RLock()
	defer n.stopMu.RUnlock()

	for _, m := range messages {
		// Process locally
		if m.To == n.Config.ID {
			if err := n.Step(n.Ctx, m); err != nil {
				return err
			}
			continue
		}

		n.asyncTasks.Add(1)
		go n.sendToMember(members, m)
	}

	return nil
}

func (n *Node) sendToMember(members map[uint64]*membership.Member, m raftpb.Message) {
	defer n.asyncTasks.Done()

	if n.cluster.IsIDRemoved(m.To) {
		// Should not send to removed members
		return
	}

	ctx, cancel := context.WithTimeout(n.Ctx, n.sendTimeout)
	defer cancel()

	var (
		conn *membership.Member
	)
	if toMember, ok := members[m.To]; ok {
		conn = toMember
	} else {
		// If we are being asked to send to a member that's not in
		// our member list, that could indicate that the current leader
		// was added while we were offline. Try to resolve its address.
		n.Config.Logger.Warningf("sending message to an unrecognized member ID %x", m.To)

		// Choose a random member
		var (
			queryMember *membership.Member
			id          uint64
		)
		for id, queryMember = range members {
			if id != n.Config.ID {
				break
			}
		}

		if queryMember == nil || queryMember.RaftID == n.Config.ID {
			n.Config.Logger.Error("could not find cluster member to query for leader address")
			return
		}

		resp, err := queryMember.ResolveAddress(ctx, &api.ResolveAddressRequest{RaftID: m.To})
		if err != nil {
			n.Config.Logger.Errorf("could not resolve address of member ID %x: %v", m.To, err)
			return
		}
		conn, err = n.ConnectToMember(resp.Addr, n.sendTimeout)
		if err != nil {
			n.Config.Logger.Errorf("could connect to member ID %x at %s: %v", m.To, resp.Addr, err)
			return
		}
		// The temporary connection is only used for this message.
		// Eventually, we should catch up and add a long-lived
		// connection to the member list.
		defer conn.Conn.Close()
	}

	_, err := conn.ProcessRaftMessage(ctx, &api.ProcessRaftMessageRequest{Message: &m})
	if err != nil {
		if grpc.ErrorDesc(err) == ErrMemberRemoved.Error() {
			n.removeRaftOnce.Do(func() {
				close(n.removeRaftCh)
			})
		}
		if m.Type == raftpb.MsgSnap {
			n.ReportSnapshot(m.To, raft.SnapshotFailure)
		}
		if n.IsStopped() {
			panic("node is nil")
		}
		n.ReportUnreachable(m.To)

		// Bounce the connection
		newConn, err := n.ConnectToMember(conn.Addr, 0)
		if err != nil {
			n.Config.Logger.Errorf("could connect to member ID %x at %s: %v", m.To, conn.Addr, err)
		} else {
			n.cluster.ReplaceMemberConnection(m.To, newConn)
		}
	} else if m.Type == raftpb.MsgSnap {
		n.ReportSnapshot(m.To, raft.SnapshotFinish)
	}
}

type applyResult struct {
	resp proto.Message
	err  error
}

// processInternalRaftRequest sends a message to nodes participating
// in the raft to apply a log entry and then waits for it to be applied
// on the server. It will block until the update is performed, there is
// an error or until the raft node finalizes all the proposals on node
// shutdown.
func (n *Node) processInternalRaftRequest(ctx context.Context, r *api.InternalRaftRequest, cb func()) (proto.Message, error) {
	n.waitProp.Add(1)
	defer n.waitProp.Done()

	if !n.canSubmitProposal() {
		return nil, ErrStopped
	}

	r.ID = n.reqIDGen.Next()

	ch := n.wait.register(r.ID, cb)

	// Do this check after calling register to avoid a race.
	if !n.IsLeader() {
		n.wait.cancel(r.ID)
		return nil, ErrLostLeadership
	}

	data, err := r.Marshal()
	if err != nil {
		n.wait.cancel(r.ID)
		return nil, err
	}

	if len(data) > store.MaxTransactionBytes {
		n.wait.cancel(r.ID)
		return nil, ErrRequestTooLarge
	}

	// This must use the context which is cancelled by stop() to avoid a
	// deadlock on shutdown.
	err = n.Propose(n.Ctx, data)
	if err != nil {
		n.wait.cancel(r.ID)
		return nil, err
	}

	select {
	case x, ok := <-ch:
		if ok {
			res := x.(*applyResult)
			return res.resp, res.err
		}
		return nil, ErrLostLeadership
	case <-n.Ctx.Done():
		n.wait.cancel(r.ID)
		return nil, ErrStopped
	case <-ctx.Done():
		n.wait.cancel(r.ID)
		return nil, ctx.Err()
	}
}

// configure sends a configuration change through consensus and
// then waits for it to be applied to the server. It will block
// until the change is performed or there is an error.
func (n *Node) configure(ctx context.Context, cc raftpb.ConfChange) error {
	cc.ID = n.reqIDGen.Next()
	ch := n.wait.register(cc.ID, nil)

	if err := n.ProposeConfChange(ctx, cc); err != nil {
		n.wait.trigger(cc.ID, nil)
		return err
	}

	select {
	case x := <-ch:
		if err, ok := x.(error); ok {
			return err
		}
		if x != nil {
			log.G(ctx).Panic("raft: configuration change error, return type should always be error")
		}
		return nil
	case <-ctx.Done():
		n.wait.trigger(cc.ID, nil)
		return ctx.Err()
	case <-n.Ctx.Done():
		return ErrStopped
	}
}

func (n *Node) processCommitted(entry raftpb.Entry) error {
	// Process a normal entry
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		if err := n.processEntry(entry); err != nil {
			return err
		}
	}

	// Process a configuration change (add/remove node)
	if entry.Type == raftpb.EntryConfChange {
		n.processConfChange(entry)
	}

	n.appliedIndex = entry.Index
	return nil
}

func (n *Node) processEntry(entry raftpb.Entry) error {
	r := &api.InternalRaftRequest{}
	err := proto.Unmarshal(entry.Data, r)
	if err != nil {
		return err
	}

	if r.Action == nil {
		return nil
	}

	if !n.wait.trigger(r.ID, &applyResult{resp: r, err: nil}) {
		// There was no wait on this ID, meaning we don't have a
		// transaction in progress that would be committed to the
		// memory store by the "trigger" call. Either a different node
		// wrote this to raft, or we wrote it before losing the leader
		// position and cancelling the transaction. Create a new
		// transaction to commit the data.

		err := n.memoryStore.ApplyStoreActions(r.Action)
		if err != nil {
			log.G(context.Background()).Errorf("error applying actions from raft: %v", err)
		}
	}
	return nil
}

func (n *Node) processConfChange(entry raftpb.Entry) {
	var (
		err error
		cc  raftpb.ConfChange
	)

	if err := proto.Unmarshal(entry.Data, &cc); err != nil {
		n.wait.trigger(cc.ID, err)
	}

	if err := n.cluster.ValidateConfigurationChange(cc); err != nil {
		n.wait.trigger(cc.ID, err)
	}

	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		err = n.applyAddNode(cc)
	case raftpb.ConfChangeRemoveNode:
		err = n.applyRemoveNode(cc)
	}

	if err != nil {
		n.wait.trigger(cc.ID, err)
	}

	n.confState = *n.ApplyConfChange(cc)
	n.wait.trigger(cc.ID, nil)
}

// applyAddNode is called when we receive a ConfChange
// from a member in the raft cluster, this adds a new
// node to the existing raft cluster
func (n *Node) applyAddNode(cc raftpb.ConfChange) error {
	member := &api.RaftMember{}
	err := proto.Unmarshal(cc.Context, member)
	if err != nil {
		return err
	}

	// ID must be non zero
	if member.RaftID == 0 {
		return nil
	}

	if err = n.registerNode(member); err != nil {
		return err
	}
	return nil
}

// applyRemoveNode is called when we receive a ConfChange
// from a member in the raft cluster, this removes a node
// from the existing raft cluster
func (n *Node) applyRemoveNode(cc raftpb.ConfChange) (err error) {
	// If the node from where the remove is issued is
	// a follower and the leader steps down, Campaign
	// to be the leader.

	if cc.NodeID == n.Leader() && !n.IsLeader() {
		if err = n.Campaign(n.Ctx); err != nil {
			return err
		}
	}

	if cc.NodeID == n.Config.ID {
		// wait the commit ack to be sent before closing connection
		n.asyncTasks.Wait()

		// if there are only 2 nodes in the cluster, and leader is leaving
		// before closing the connection, leader has to ensure that follower gets
		// noticed about this raft conf change commit. Otherwise, follower would
		// assume there are still 2 nodes in the cluster and won't get elected
		// into the leader by acquiring the majority (2 nodes)

		// while n.asyncTasks.Wait() could be helpful in this case
		// it's the best-effort strategy, because this send could be fail due to some errors (such as time limit exceeds)
		// TODO(Runshen Zhu): use leadership transfer to solve this case, after vendoring raft 3.0+
	}

	return n.cluster.RemoveMember(cc.NodeID)
}

// ConnectToMember returns a member object with an initialized
// connection to communicate with other raft members
func (n *Node) ConnectToMember(addr string, timeout time.Duration) (*membership.Member, error) {
	conn, err := dial(addr, "tcp", n.tlsCredentials, timeout)
	if err != nil {
		return nil, err
	}

	return &membership.Member{
		RaftClient: api.NewRaftClient(conn),
		Conn:       conn,
	}, nil
}

// SubscribeLeadership returns channel to which events about leadership change
// will be sent in form of raft.LeadershipState. Also cancel func is returned -
// it should be called when listener is not longer interested in events.
func (n *Node) SubscribeLeadership() (q chan events.Event, cancel func()) {
	ch := events.NewChannel(0)
	sink := events.Sink(events.NewQueue(ch))
	n.leadershipBroadcast.Add(sink)
	return ch.C, func() {
		n.leadershipBroadcast.Remove(sink)
		ch.Close()
		sink.Close()
	}
}

// createConfigChangeEnts creates a series of Raft entries (i.e.
// EntryConfChange) to remove the set of given IDs from the cluster. The ID
// `self` is _not_ removed, even if present in the set.
// If `self` is not inside the given ids, it creates a Raft entry to add a
// default member with the given `self`.
func createConfigChangeEnts(ids []uint64, self uint64, term, index uint64) []raftpb.Entry {
	var ents []raftpb.Entry
	next := index + 1
	found := false
	for _, id := range ids {
		if id == self {
			found = true
			continue
		}
		cc := &raftpb.ConfChange{
			Type:   raftpb.ConfChangeRemoveNode,
			NodeID: id,
		}
		data, err := cc.Marshal()
		if err != nil {
			log.G(context.Background()).Panicf("marshal configuration change should never fail: %v", err)
		}
		e := raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Data:  data,
			Term:  term,
			Index: next,
		}
		ents = append(ents, e)
		next++
	}
	if !found {
		node := &api.RaftMember{RaftID: self}
		meta, err := node.Marshal()
		if err != nil {
			log.G(context.Background()).Panicf("marshal member should never fail: %v", err)
		}
		cc := &raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  self,
			Context: meta,
		}
		data, err := cc.Marshal()
		if err != nil {
			log.G(context.Background()).Panicf("marshal configuration change should never fail: %v", err)
		}
		e := raftpb.Entry{
			Type:  raftpb.EntryConfChange,
			Data:  data,
			Term:  term,
			Index: next,
		}
		ents = append(ents, e)
	}
	return ents
}

// getIDs returns an ordered set of IDs included in the given snapshot and
// the entries. The given snapshot/entries can contain two kinds of
// ID-related entry:
// - ConfChangeAddNode, in which case the contained ID will be added into the set.
// - ConfChangeRemoveNode, in which case the contained ID will be removed from the set.
func getIDs(snap *raftpb.Snapshot, ents []raftpb.Entry) []uint64 {
	ids := make(map[uint64]bool)
	if snap != nil {
		for _, id := range snap.Metadata.ConfState.Nodes {
			ids[id] = true
		}
	}
	for _, e := range ents {
		if e.Type != raftpb.EntryConfChange {
			continue
		}
		if snap != nil && e.Index < snap.Metadata.Index {
			continue
		}
		var cc raftpb.ConfChange
		if err := cc.Unmarshal(e.Data); err != nil {
			log.G(context.Background()).Panicf("unmarshal configuration change should never fail: %v", err)
		}
		switch cc.Type {
		case raftpb.ConfChangeAddNode:
			ids[cc.NodeID] = true
		case raftpb.ConfChangeRemoveNode:
			delete(ids, cc.NodeID)
		case raftpb.ConfChangeUpdateNode:
			// do nothing
		default:
			log.G(context.Background()).Panic("ConfChange Type should be either ConfChangeAddNode or ConfChangeRemoveNode!")
		}
	}
	var sids []uint64
	for id := range ids {
		sids = append(sids, id)
	}
	return sids
}
