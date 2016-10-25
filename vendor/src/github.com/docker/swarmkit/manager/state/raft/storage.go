package raft

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/raft/membership"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var errNoWAL = errors.New("no WAL present")

func (n *Node) legacyWALDir() string {
	return filepath.Join(n.opts.StateDir, "wal")
}

func (n *Node) walDir() string {
	return filepath.Join(n.opts.StateDir, "wal-v3")
}

func (n *Node) legacySnapDir() string {
	return filepath.Join(n.opts.StateDir, "snap")
}

func (n *Node) snapDir() string {
	return filepath.Join(n.opts.StateDir, "snap-v3")
}

func (n *Node) loadAndStart(ctx context.Context, forceNewCluster bool) error {
	walDir := n.walDir()
	snapDir := n.snapDir()

	if !fileutil.Exist(snapDir) {
		// If snapshots created by the etcd-v2 code exist, hard link
		// them at the new path. This prevents etc-v2 creating
		// snapshots that are visible to us, but out of sync with our
		// WALs, after a downgrade.
		legacySnapDir := n.legacySnapDir()
		if fileutil.Exist(legacySnapDir) {
			if err := migrateSnapshots(legacySnapDir, snapDir); err != nil {
				return err
			}
		} else if err := os.MkdirAll(snapDir, 0700); err != nil {
			return errors.Wrap(err, "failed to create snapshot directory")
		}
	}

	// Create a snapshotter
	n.snapshotter = snap.New(snapDir)

	if !wal.Exist(walDir) {
		// If wals created by the etcd-v2 wal code exist, copy them to
		// the new path to avoid adding backwards-incompatible entries
		// to those files.
		legacyWALDir := n.legacyWALDir()
		if !wal.Exist(legacyWALDir) {
			return errNoWAL
		}

		if err := migrateWALs(legacyWALDir, walDir); err != nil {
			return err
		}
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

	return nil
}

func migrateWALs(legacyWALDir, walDir string) error {
	// keep temporary wal directory so WAL initialization appears atomic
	tmpdirpath := filepath.Clean(walDir) + ".tmp"
	if fileutil.Exist(tmpdirpath) {
		if err := os.RemoveAll(tmpdirpath); err != nil {
			return errors.Wrap(err, "could not remove temporary WAL directory")
		}
	}
	if err := fileutil.CreateDirAll(tmpdirpath); err != nil {
		return errors.Wrap(err, "could not create temporary WAL directory")
	}

	walNames, err := fileutil.ReadDir(legacyWALDir)
	if err != nil {
		return errors.Wrapf(err, "could not list WAL directory %s", legacyWALDir)
	}

	for _, fname := range walNames {
		_, err := copyFile(filepath.Join(legacyWALDir, fname), filepath.Join(tmpdirpath, fname), 0600)
		if err != nil {
			return errors.Wrap(err, "error copying WAL file")
		}
	}

	if err := os.Rename(tmpdirpath, walDir); err != nil {
		return err
	}

	return nil
}

func migrateSnapshots(legacySnapDir, snapDir string) error {
	// use temporary snaphot directory so initialization appears atomic
	tmpdirpath := filepath.Clean(snapDir) + ".tmp"
	if fileutil.Exist(tmpdirpath) {
		if err := os.RemoveAll(tmpdirpath); err != nil {
			return errors.Wrap(err, "could not remove temporary snapshot directory")
		}
	}
	if err := fileutil.CreateDirAll(tmpdirpath); err != nil {
		return errors.Wrap(err, "could not create temporary snapshot directory")
	}

	snapshotNames, err := fileutil.ReadDir(legacySnapDir)
	if err != nil {
		return errors.Wrapf(err, "could not list snapshot directory %s", legacySnapDir)
	}

	for _, fname := range snapshotNames {
		err := os.Link(filepath.Join(legacySnapDir, fname), filepath.Join(tmpdirpath, fname))
		if err != nil {
			return errors.Wrap(err, "error linking snapshot file")
		}
	}

	if err := os.Rename(tmpdirpath, snapDir); err != nil {
		return err
	}

	return nil
}

// copyFile copies from src to dst until either EOF is reached
// on src or an error occurs. It verifies src exists and removes
// the dst if it exists.
func copyFile(src, dst string, perm os.FileMode) (int64, error) {
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)
	if cleanSrc == cleanDst {
		return 0, nil
	}
	sf, err := os.Open(cleanSrc)
	if err != nil {
		return 0, err
	}
	defer sf.Close()
	if err := os.Remove(cleanDst); err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	df, err := os.OpenFile(cleanDst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return 0, err
	}
	defer df.Close()
	return io.Copy(df, sf)
}

func (n *Node) createWAL(nodeID string) (raft.Peer, error) {
	raftNode := &api.RaftMember{
		RaftID: n.Config.ID,
		NodeID: nodeID,
		Addr:   n.opts.Addr,
	}
	metadata, err := raftNode.Marshal()
	if err != nil {
		return raft.Peer{}, errors.Wrap(err, "error marshalling raft node")
	}
	n.wal, err = wal.Create(n.walDir(), metadata)
	if err != nil {
		return raft.Peer{}, errors.Wrap(err, "failed to create WAL")
	}

	n.cluster.AddMember(&membership.Member{RaftMember: raftNode})
	return raft.Peer{ID: n.Config.ID, Context: metadata}, nil
}

// moveWALAndSnap moves away the WAL and snapshot because we were removed
// from the cluster and will need to recreate them if we are readded.
func (n *Node) moveWALAndSnap() error {
	newWALDir, err := ioutil.TempDir(n.opts.StateDir, "wal.")
	if err != nil {
		return err
	}
	err = os.Rename(n.walDir(), newWALDir)
	if err != nil {
		return err
	}

	newSnapDir, err := ioutil.TempDir(n.opts.StateDir, "snap.")
	if err != nil {
		return err
	}
	err = os.Rename(n.snapDir(), newSnapDir)
	if err != nil {
		return err
	}

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
			return errors.Wrap(err, "failed to open WAL")
		}
		if metadata, st, ents, err = n.wal.ReadAll(); err != nil {
			if err := n.wal.Close(); err != nil {
				return err
			}
			// we can only repair ErrUnexpectedEOF and we never repair twice.
			if repaired || err != io.ErrUnexpectedEOF {
				return errors.Wrap(err, "irreparable WAL error")
			}
			if !wal.Repair(n.walDir()) {
				return errors.Wrap(err, "WAL error cannot be repaired")
			}
			log.G(ctx).WithError(err).Info("repaired WAL error")
			repaired = true
			continue
		}
		break
	}

	defer func() {
		if err != nil {
			if walErr := n.wal.Close(); walErr != nil {
				log.G(ctx).WithError(walErr).Error("error closing raft WAL")
			}
		}
	}()

	var raftNode api.RaftMember
	if err := raftNode.Unmarshal(metadata); err != nil {
		return errors.Wrap(err, "failed to unmarshal WAL metadata")
	}
	n.Config.ID = raftNode.RaftID

	// All members that are no longer part of the cluster must be added to
	// the removed list right away, so that we don't try to connect to them
	// before processing the configuration change entries, which could make
	// us get stuck.
	for _, ent := range ents {
		if ent.Index <= st.Commit && ent.Type == raftpb.EntryConfChange {
			var cc raftpb.ConfChange
			if err := cc.Unmarshal(ent.Data); err != nil {
				return errors.Wrap(err, "failed to unmarshal config change")
			}
			if cc.Type == raftpb.ConfChangeRemoveNode {
				n.cluster.RemoveMember(cc.NodeID)
			}
		}
	}

	if forceNewCluster {
		// discard the previously uncommitted entries
		for i, ent := range ents {
			if ent.Index > st.Commit {
				log.G(ctx).Infof("discarding %d uncommitted WAL entries ", len(ents)-i)
				ents = ents[:i]
				break
			}
		}

		// force append the configuration change entries
		toAppEnts := createConfigChangeEnts(getIDs(snapshot, ents), uint64(n.Config.ID), st.Term, st.Commit)

		// All members that are being removed as part of the
		// force-new-cluster process must be added to the
		// removed list right away, so that we don't try to
		// connect to them before processing the configuration
		// change entries, which could make us get stuck.
		for _, ccEnt := range toAppEnts {
			if ccEnt.Type == raftpb.EntryConfChange {
				var cc raftpb.ConfChange
				if err := cc.Unmarshal(ccEnt.Data); err != nil {
					return errors.Wrap(err, "error unmarshalling force-new-cluster config change")
				}
				if cc.Type == raftpb.ConfChangeRemoveNode {
					n.cluster.RemoveMember(cc.NodeID)
				}
			}
		}
		ents = append(ents, toAppEnts...)

		// force commit newly appended entries
		err := n.wal.Save(st, toAppEnts)
		if err != nil {
			log.G(ctx).WithError(err).Fatalf("failed to save WAL while forcing new cluster")
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
	curSnapshotIdx := -1
	var (
		removeErr      error
		oldestSnapshot string
	)

	for i, snapFile := range snapshots {
		if curSnapshotIdx >= 0 && i > curSnapshotIdx {
			if uint64(i-curSnapshotIdx) > keepOldSnapshots {
				err := os.Remove(filepath.Join(n.snapDir(), snapFile))
				if err != nil && removeErr == nil {
					removeErr = err
				}
				continue
			}
		} else if snapFile == curSnapshot {
			curSnapshotIdx = i
		}
		oldestSnapshot = snapFile
	}

	if removeErr != nil {
		return removeErr
	}

	// Remove any WAL files that only contain data from before the oldest
	// remaining snapshot.

	if oldestSnapshot == "" {
		return nil
	}

	// Parse index out of oldest snapshot's filename
	var snapTerm, snapIndex uint64
	_, err = fmt.Sscanf(oldestSnapshot, "%016x-%016x.snap", &snapTerm, &snapIndex)
	if err != nil {
		return errors.Wrapf(err, "malformed snapshot filename %s", oldestSnapshot)
	}

	// List the WALs
	dirents, err = ioutil.ReadDir(n.walDir())
	if err != nil {
		return err
	}

	var wals []string
	for _, dirent := range dirents {
		if strings.HasSuffix(dirent.Name(), ".wal") {
			wals = append(wals, dirent.Name())
		}
	}

	// Sort WAL filenames in lexical order
	sort.Sort(sort.StringSlice(wals))

	found := false
	deleteUntil := -1

	for i, walName := range wals {
		var walSeq, walIndex uint64
		_, err = fmt.Sscanf(walName, "%016x-%016x.wal", &walSeq, &walIndex)
		if err != nil {
			return errors.Wrapf(err, "could not parse WAL name %s", walName)
		}

		if walIndex >= snapIndex {
			deleteUntil = i - 1
			found = true
			break
		}
	}

	// If all WAL files started with indices below the oldest snapshot's
	// index, we can delete all but the newest WAL file.
	if !found && len(wals) != 0 {
		deleteUntil = len(wals) - 1
	}

	for i := 0; i < deleteUntil; i++ {
		walPath := filepath.Join(n.walDir(), wals[i])
		l, err := fileutil.TryLockFile(walPath, os.O_WRONLY, fileutil.PrivateFileMode)
		if err != nil {
			return errors.Wrapf(err, "could not lock old WAL file %s for removal", wals[i])
		}
		err = os.Remove(walPath)
		l.Close()
		if err != nil {
			return errors.Wrapf(err, "error removing old WAL file %s", wals[i])
		}
	}

	return nil
}

func (n *Node) doSnapshot(ctx context.Context, raftConfig api.RaftConfig) {
	snapshot := api.Snapshot{Version: api.Snapshot_V0}
	for _, member := range n.cluster.Members() {
		snapshot.Membership.Members = append(snapshot.Membership.Members,
			&api.RaftMember{
				NodeID: member.NodeID,
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
			log.G(ctx).WithError(err).Error("failed to read snapshot from store")
			return
		}

		d, err := snapshot.Marshal()
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to marshal snapshot")
			return
		}
		snap, err := n.raftStore.CreateSnapshot(appliedIndex, &n.confState, d)
		if err == nil {
			if err := n.saveSnapshot(snap, raftConfig.KeepOldSnapshots); err != nil {
				log.G(ctx).WithError(err).Error("failed to save snapshot")
				return
			}
			snapshotIndex = appliedIndex

			if appliedIndex > raftConfig.LogEntriesForSlowFollowers {
				err := n.raftStore.Compact(appliedIndex - raftConfig.LogEntriesForSlowFollowers)
				if err != nil && err != raft.ErrCompacted {
					log.G(ctx).WithError(err).Error("failed to compact snapshot")
				}
			}
		} else if err != raft.ErrSnapOutOfDate {
			log.G(ctx).WithError(err).Error("failed to create snapshot")
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

	oldMembers := n.cluster.Members()

	if !forceNewCluster {
		for _, member := range snapshot.Membership.Members {
			if err := n.registerNode(&api.RaftMember{RaftID: member.RaftID, NodeID: member.NodeID, Addr: member.Addr}); err != nil {
				return err
			}
			delete(oldMembers, member.RaftID)
		}
	}

	for _, removedMember := range snapshot.Membership.Removed {
		n.cluster.RemoveMember(removedMember)
		delete(oldMembers, removedMember)
	}

	for member := range oldMembers {
		n.cluster.ClearMember(member)
	}

	return nil
}
