package raft

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/raft/membership"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

func (n *Node) walDir() string {
	return filepath.Join(n.StateDir, "wal")
}

func (n *Node) snapDir() string {
	return filepath.Join(n.StateDir, "snap")
}

func (n *Node) loadAndStart(ctx context.Context, forceNewCluster bool) error {
	walDir := n.walDir()
	snapDir := n.snapDir()

	if err := os.MkdirAll(snapDir, 0700); err != nil {
		return fmt.Errorf("create snapshot directory error: %v", err)
	}

	// Create a snapshotter
	n.snapshotter = snap.New(snapDir)

	if !wal.Exist(walDir) {
		raftNode := &api.RaftMember{
			RaftID: n.Config.ID,
			Addr:   n.Address,
		}
		metadata, err := raftNode.Marshal()
		if err != nil {
			return fmt.Errorf("error marshalling raft node: %v", err)
		}
		n.wal, err = wal.Create(walDir, metadata)
		if err != nil {
			return fmt.Errorf("create wal error: %v", err)
		}

		n.cluster.AddMember(&membership.Member{RaftMember: raftNode})
		n.startNodePeers = []raft.Peer{{ID: n.Config.ID, Context: metadata}}

		return nil
	}

	// Load snapshot data
	snapshot, err := n.snapshotter.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		return err
	}

	if snapshot != nil {
		// Load the snapshot data into the store
		if err := n.restoreFromSnapshot(snapshot.Data, forceNewCluster); err != nil {
			return err
		}
	}

	// Read logs to fully catch up store
	if err := n.readWAL(ctx, snapshot, forceNewCluster); err != nil {
		return err
	}

	n.Node = raft.RestartNode(n.Config)
	return nil
}

func (n *Node) readWAL(ctx context.Context, snapshot *raftpb.Snapshot, forceNewCluster bool) (err error) {
	var (
		walsnap  walpb.Snapshot
		metadata []byte
		st       raftpb.HardState
		ents     []raftpb.Entry
	)

	if snapshot != nil {
		walsnap.Index = snapshot.Metadata.Index
		walsnap.Term = snapshot.Metadata.Term
	}

	repaired := false
	for {
		if n.wal, err = wal.Open(n.walDir(), walsnap); err != nil {
			return fmt.Errorf("open wal error: %v", err)
		}
		if metadata, st, ents, err = n.wal.ReadAll(); err != nil {
			if err := n.wal.Close(); err != nil {
				return err
			}
			// we can only repair ErrUnexpectedEOF and we never repair twice.
			if repaired || err != io.ErrUnexpectedEOF {
				return fmt.Errorf("read wal error (%v) and cannot be repaired", err)
			}
			if !wal.Repair(n.walDir()) {
				return fmt.Errorf("WAL error (%v) cannot be repaired", err)
			}
			log.G(ctx).Infof("repaired WAL error (%v)", err)
			repaired = true
			continue
		}
		break
	}

	defer func() {
		if err != nil {
			if walErr := n.wal.Close(); walErr != nil {
				n.Config.Logger.Errorf("error closing raft WAL: %v", walErr)
			}
		}
	}()

	var raftNode api.RaftMember
	if err := raftNode.Unmarshal(metadata); err != nil {
		return fmt.Errorf("error unmarshalling wal metadata: %v", err)
	}
	n.Config.ID = raftNode.RaftID

	if forceNewCluster {
		// discard the previously uncommitted entries
		for i, ent := range ents {
			if ent.Index > st.Commit {
				log.G(context.Background()).Infof("discarding %d uncommitted WAL entries ", len(ents)-i)
				ents = ents[:i]
				break
			}
		}

		// force append the configuration change entries
		toAppEnts := createConfigChangeEnts(getIDs(snapshot, ents), uint64(n.Config.ID), st.Term, st.Commit)
		ents = append(ents, toAppEnts...)

		// force commit newly appended entries
		err := n.wal.Save(st, toAppEnts)
		if err != nil {
			log.G(context.Background()).Fatalf("%v", err)
		}
		if len(toAppEnts) != 0 {
			st.Commit = toAppEnts[len(toAppEnts)-1].Index
		}
	}

	if snapshot != nil {
		if err := n.raftStore.ApplySnapshot(*snapshot); err != nil {
			return err
		}
	}
	if err := n.raftStore.SetHardState(st); err != nil {
		return err
	}
	if err := n.raftStore.Append(ents); err != nil {
		return err
	}

	return nil
}

func (n *Node) saveSnapshot(snapshot raftpb.Snapshot, keepOldSnapshots uint64) error {
	err := n.wal.SaveSnapshot(walpb.Snapshot{
		Index: snapshot.Metadata.Index,
		Term:  snapshot.Metadata.Term,
	})
	if err != nil {
		return err
	}
	err = n.snapshotter.SaveSnap(snapshot)
	if err != nil {
		return err
	}
	err = n.wal.ReleaseLockTo(snapshot.Metadata.Index)
	if err != nil {
		return err
	}

	// Delete any older snapshots
	curSnapshot := fmt.Sprintf("%016x-%016x%s", snapshot.Metadata.Term, snapshot.Metadata.Index, ".snap")

	dirents, err := ioutil.ReadDir(n.snapDir())
	if err != nil {
		return err
	}

	var snapshots []string
	for _, dirent := range dirents {
		if strings.HasSuffix(dirent.Name(), ".snap") {
			snapshots = append(snapshots, dirent.Name())
		}
	}

	// Sort snapshot filenames in reverse lexical order
	sort.Sort(sort.Reverse(sort.StringSlice(snapshots)))

	// Ignore any snapshots that are older than the current snapshot.
	// Delete the others. Rather than doing lexical comparisons, we look
	// at what exists before/after the current snapshot in the slice.
	// This means that if the current snapshot doesn't appear in the
	// directory for some strange reason, we won't delete anything, which
	// is the safe behavior.
	var (
		afterCurSnapshot bool
		removeErr        error
	)
	for i, snapFile := range snapshots {
		if afterCurSnapshot {
			if uint64(len(snapshots)-i) <= keepOldSnapshots {
				return removeErr
			}
			err := os.Remove(filepath.Join(n.snapDir(), snapFile))
			if err != nil && removeErr == nil {
				removeErr = err
			}
		} else if snapFile == curSnapshot {
			afterCurSnapshot = true
		}
	}

	return removeErr
}

func (n *Node) doSnapshot(raftConfig *api.RaftConfig) {
	snapshot := api.Snapshot{Version: api.Snapshot_V0}
	for _, member := range n.cluster.Members() {
		snapshot.Membership.Members = append(snapshot.Membership.Members,
			&api.RaftMember{
				RaftID: member.RaftID,
				Addr:   member.Addr,
			})
	}
	snapshot.Membership.Removed = n.cluster.Removed()

	viewStarted := make(chan struct{})
	n.asyncTasks.Add(1)
	n.snapshotInProgress = make(chan uint64, 1) // buffered in case Shutdown is called during the snapshot
	go func(appliedIndex, snapshotIndex uint64) {
		defer func() {
			n.asyncTasks.Done()
			n.snapshotInProgress <- snapshotIndex
		}()

		var err error
		n.memoryStore.View(func(tx store.ReadTx) {
			close(viewStarted)

			var storeSnapshot *api.StoreSnapshot
			storeSnapshot, err = n.memoryStore.Save(tx)
			snapshot.Store = *storeSnapshot
		})
		if err != nil {
			n.Config.Logger.Error(err)
			return
		}

		d, err := snapshot.Marshal()
		if err != nil {
			n.Config.Logger.Error(err)
			return
		}
		snap, err := n.raftStore.CreateSnapshot(appliedIndex, &n.confState, d)
		if err == nil {
			if err := n.saveSnapshot(snap, raftConfig.KeepOldSnapshots); err != nil {
				n.Config.Logger.Error(err)
				return
			}
			snapshotIndex = appliedIndex

			if appliedIndex > raftConfig.LogEntriesForSlowFollowers {
				err := n.raftStore.Compact(appliedIndex - raftConfig.LogEntriesForSlowFollowers)
				if err != nil && err != raft.ErrCompacted {
					n.Config.Logger.Error(err)
				}
			}
		} else if err != raft.ErrSnapOutOfDate {
			n.Config.Logger.Error(err)
		}
	}(n.appliedIndex, n.snapshotIndex)

	// Wait for the goroutine to establish a read transaction, to make
	// sure it sees the state as of this moment.
	<-viewStarted
}

func (n *Node) restoreFromSnapshot(data []byte, forceNewCluster bool) error {
	var snapshot api.Snapshot
	if err := snapshot.Unmarshal(data); err != nil {
		return err
	}
	if snapshot.Version != api.Snapshot_V0 {
		return fmt.Errorf("unrecognized snapshot version %d", snapshot.Version)
	}

	if err := n.memoryStore.Restore(&snapshot.Store); err != nil {
		return err
	}

	n.cluster.Clear()

	if !forceNewCluster {
		for _, member := range snapshot.Membership.Members {
			if err := n.registerNode(&api.RaftMember{RaftID: member.RaftID, Addr: member.Addr}); err != nil {
				return err
			}
		}
		for _, removedMember := range snapshot.Membership.Removed {
			n.cluster.RemoveMember(removedMember)
		}
	}

	return nil
}
