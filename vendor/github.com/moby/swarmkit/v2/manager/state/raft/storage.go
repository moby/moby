package raft

import (
	"context"
	"fmt"

	"github.com/docker/go-metrics"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/encryption"
	"github.com/moby/swarmkit/v2/manager/state/raft/membership"
	"github.com/moby/swarmkit/v2/manager/state/raft/storage"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/pkg/errors"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
)

var (
	// Snapshot create latency timer.
	snapshotLatencyTimer metrics.Timer
)

func init() {
	ns := metrics.NewNamespace("swarm", "raft", nil)
	snapshotLatencyTimer = ns.NewTimer("snapshot_latency",
		"Raft snapshot create latency.")
	metrics.Register(ns)
}

func (n *Node) readFromDisk(ctx context.Context) (*raftpb.Snapshot, storage.WALData, error) {
	keys := n.keyRotator.GetKeys()

	n.raftLogger = &storage.EncryptedRaftLogger{
		StateDir:      n.opts.StateDir,
		EncryptionKey: keys.CurrentDEK,
		FIPS:          n.opts.FIPS,
	}
	if keys.PendingDEK != nil {
		n.raftLogger.EncryptionKey = keys.PendingDEK
	}

	snap, walData, err := n.raftLogger.BootstrapFromDisk(ctx)

	if keys.PendingDEK != nil {
		switch errors.Cause(err).(type) {
		case nil:
			if err = n.keyRotator.UpdateKeys(EncryptionKeys{CurrentDEK: keys.PendingDEK}); err != nil {
				err = errors.Wrap(err, "previous key rotation was successful, but unable mark rotation as complete")
			}
		case encryption.ErrCannotDecrypt:
			snap, walData, err = n.raftLogger.BootstrapFromDisk(ctx, keys.CurrentDEK)
		}
	}

	if err != nil {
		return nil, storage.WALData{}, err
	}
	return snap, walData, nil
}

// bootstraps a node's raft store from the raft logs and snapshots on disk
func (n *Node) loadAndStart(ctx context.Context, forceNewCluster bool) error {
	snapshot, waldata, err := n.readFromDisk(ctx)
	if err != nil {
		return err
	}

	// Read logs to fully catch up store
	var raftNode api.RaftMember
	if err := raftNode.UnmarshalVT(waldata.Metadata); err != nil {
		return errors.Wrap(err, "failed to unmarshal WAL metadata")
	}
	n.Config.ID = raftNode.RaftId

	if snapshot != nil {
		snapCluster, err := n.clusterSnapshot(snapshot.Data)
		if err != nil {
			return err
		}
		var bootstrapMembers []*api.RaftMember
		if forceNewCluster {
			for _, m := range snapCluster.GetMembers() {
				if m.RaftId != n.Config.ID {
					n.cluster.RemoveMember(m.RaftId)
					continue
				}
				bootstrapMembers = append(bootstrapMembers, m)
			}
		} else {
			bootstrapMembers = snapCluster.GetMembers()
		}
		n.bootstrapMembers = bootstrapMembers
		for _, removedMember := range snapCluster.GetRemoved() {
			n.cluster.RemoveMember(removedMember)
		}
	}

	ents, st := waldata.Entries, waldata.HardState

	// All members that are no longer part of the cluster must be added to
	// the removed list right away, so that we don't try to connect to them
	// before processing the configuration change entries, which could make
	// us get stuck.
	for _, ent := range ents {
		if ent.GetIndex() <= st.GetCommit() && ent.GetType() == raftpb.EntryConfChange {
			var cc raftpb.ConfChange
			if err := proto.Unmarshal(ent.Data, &cc); err != nil {
				return errors.Wrap(err, "failed to unmarshal config change")
			}
			if cc.GetType() == raftpb.ConfChangeRemoveNode {
				n.cluster.RemoveMember(cc.GetNodeId())
			}
		}
	}

	if forceNewCluster {
		// discard the previously uncommitted entries
		for i, ent := range ents {
			if ent.GetIndex() > st.GetCommit() {
				log.G(ctx).Infof("discarding %d uncommitted WAL entries", len(ents)-i)
				ents = ents[:i]
				break
			}
		}

		// force append the configuration change entries
		toAppEnts := createConfigChangeEnts(getIDs(snapshot, ents), n.Config.ID, st.GetTerm(), st.GetCommit())

		// All members that are being removed as part of the
		// force-new-cluster process must be added to the
		// removed list right away, so that we don't try to
		// connect to them before processing the configuration
		// change entries, which could make us get stuck.
		for _, ccEnt := range toAppEnts {
			if ccEnt.GetType() == raftpb.EntryConfChange {
				var cc raftpb.ConfChange
				if err := proto.Unmarshal(ccEnt.Data, &cc); err != nil {
					return errors.Wrap(err, "error unmarshalling force-new-cluster config change")
				}
				if cc.GetType() == raftpb.ConfChangeRemoveNode {
					n.cluster.RemoveMember(cc.GetNodeId())
				}
			}
		}
		ents = append(ents, toAppEnts...)

		// force commit newly appended entries
		err := n.raftLogger.SaveEntries(st, toAppEnts)
		if err != nil {
			log.G(ctx).WithError(err).Fatal("failed to save WAL while forcing new cluster")
		}
		if len(toAppEnts) != 0 {
			st.Commit = new(toAppEnts[len(toAppEnts)-1].GetIndex())
		}
	}

	if snapshot != nil {
		if err := n.raftStore.ApplySnapshot(snapshot); err != nil {
			return err
		}
	}
	if err := n.raftStore.SetHardState(st); err != nil {
		return err
	}
	return n.raftStore.Append(ents)
}

func (n *Node) newRaftLogs(nodeID string) (raft.Peer, error) {
	raftNode := &api.RaftMember{
		RaftId: n.Config.ID,
		NodeId: nodeID,
		Addr:   n.opts.Addr,
	}
	metadata, err := raftNode.MarshalVT()
	if err != nil {
		return raft.Peer{}, errors.Wrap(err, "error marshalling raft node")
	}
	if err := n.raftLogger.BootstrapNew(metadata); err != nil {
		return raft.Peer{}, err
	}
	n.cluster.AddMember(&membership.Member{RaftMember: raftNode})
	return raft.Peer{ID: n.Config.ID, Context: metadata}, nil
}

func (n *Node) triggerSnapshot(ctx context.Context, raftConfig *api.RaftConfig) {
	snapshot := &api.Snapshot{
		Version:    api.Snapshot_V0,
		Membership: &api.ClusterSnapshot{},
	}
	for _, member := range n.cluster.Members() {
		snapshot.Membership.Members = append(snapshot.Membership.Members,
			&api.RaftMember{
				NodeId: member.NodeId,
				RaftId: member.RaftId,
				Addr:   member.Addr,
			})
	}
	snapshot.Membership.Removed = n.cluster.Removed()

	viewStarted := make(chan struct{})
	n.asyncTasks.Add(1)
	n.snapshotInProgress = make(chan *raftpb.SnapshotMetadata, 1) // buffered in case Shutdown is called during the snapshot
	go func(appliedIndex uint64, snapshotMeta *raftpb.SnapshotMetadata) {
		// Deferred latency capture.
		defer metrics.StartTimer(snapshotLatencyTimer)()

		defer func() {
			n.asyncTasks.Done()
			n.snapshotInProgress <- snapshotMeta
		}()
		var err error
		n.memoryStore.View(func(tx store.ReadTx) {
			close(viewStarted)

			var storeSnapshot *api.StoreSnapshot
			storeSnapshot, err = n.memoryStore.Save(tx)
			snapshot.Store = storeSnapshot
		})
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to read snapshot from store")
			return
		}

		d, err := snapshot.MarshalVT()
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to marshal snapshot")
			return
		}
		snap, err := n.raftStore.CreateSnapshot(appliedIndex, n.confState, d)
		if err == nil {
			if err := n.raftLogger.SaveSnapshot(snap); err != nil {
				log.G(ctx).WithError(err).Error("failed to save snapshot")
				return
			}
			snapshotMeta = snap.Metadata

			if appliedIndex > raftConfig.LogEntriesForSlowFollowers {
				err := n.raftStore.Compact(appliedIndex - raftConfig.LogEntriesForSlowFollowers)
				if err != nil && err != raft.ErrCompacted {
					log.G(ctx).WithError(err).Error("failed to compact snapshot")
				}
			}
		} else if err != raft.ErrSnapOutOfDate {
			log.G(ctx).WithError(err).Error("failed to create snapshot")
		}
	}(n.appliedIndex, n.snapshotMeta)

	// Wait for the goroutine to establish a read transaction, to make
	// sure it sees the state as of this moment.
	<-viewStarted
}

func (n *Node) clusterSnapshot(data []byte) (*api.ClusterSnapshot, error) {
	var snapshot api.Snapshot
	if err := snapshot.UnmarshalVT(data); err != nil {
		return snapshot.Membership, err
	}
	if snapshot.Version != api.Snapshot_V0 {
		return snapshot.Membership, fmt.Errorf("unrecognized snapshot version %d", snapshot.Version)
	}

	// Snapshot.store used to be a non-nullable field. A snapshot that arrives
	// (from a peer or from disk) without one has to clear the store, exactly as
	// an empty StoreSnapshot did, rather than panic in the restore path.
	storeSnapshot := snapshot.Store
	if storeSnapshot == nil {
		storeSnapshot = &api.StoreSnapshot{}
	}
	if err := n.memoryStore.Restore(storeSnapshot); err != nil {
		return snapshot.Membership, err
	}

	return snapshot.Membership, nil
}
