package storage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/swarmkit/v2/manager/encryption"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
)

// This package wraps the go.etcd.io/etcd/server/v3/api/snap package, and encrypts
// the bytes of whatever snapshot is passed to it, and decrypts the bytes of
// whatever snapshot it reads.

// Snapshotter is the interface presented by go.etcd.io/etcd/server/v3/api/snap.Snapshotter that we depend upon
type Snapshotter interface {
	SaveSnap(snapshot raftpb.Snapshot) error
	Load() (*raftpb.Snapshot, error)
}

// SnapFactory provides an interface for the different ways to get a Snapshotter object.
// For instance, the etcd/snap package itself provides this
type SnapFactory interface {
	New(dirpath string) Snapshotter
}

var _ Snapshotter = &wrappedSnap{}
var _ Snapshotter = &snap.Snapshotter{}
var _ SnapFactory = snapCryptor{}

// wrappedSnap wraps a go.etcd.io/etcd/server/v3/api/snap.Snapshotter, and handles
// encrypting/decrypting.
type wrappedSnap struct {
	*snap.Snapshotter
	encrypter encryption.Encrypter
	decrypter encryption.Decrypter
}

// SaveSnap encrypts the snapshot data (if an encrypter is exists) before passing it onto the
// wrapped snap.Snapshotter's SaveSnap function.
func (s *wrappedSnap) SaveSnap(snapshot raftpb.Snapshot) error {
	toWrite := snapshot
	var err error
	toWrite.Data, err = encryption.Encrypt(snapshot.Data, s.encrypter)
	if err != nil {
		return err
	}
	return s.Snapshotter.SaveSnap(toWrite)
}

// Load decrypts the snapshot data (if a decrypter is exists) after reading it using the
// wrapped snap.Snapshotter's Load function.
func (s *wrappedSnap) Load() (*raftpb.Snapshot, error) {
	snapshot, err := s.Snapshotter.Load()
	if err != nil {
		return nil, err
	}
	snapshot.Data, err = encryption.Decrypt(snapshot.Data, s.decrypter)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// snapCryptor is an object that provides the same functions as `etcd/wal`
// and `etcd/snap` that we need to open a WAL object or Snapshotter object
type snapCryptor struct {
	encrypter encryption.Encrypter
	decrypter encryption.Decrypter
}

// NewSnapFactory returns a new object that can read from and write to encrypted
// snapshots on disk
func NewSnapFactory(encrypter encryption.Encrypter, decrypter encryption.Decrypter) SnapFactory {
	return snapCryptor{
		encrypter: encrypter,
		decrypter: decrypter,
	}
}

// NewSnapshotter returns a new Snapshotter with the given encrypters and decrypters
func (sc snapCryptor) New(dirpath string) Snapshotter {
	return &wrappedSnap{
		Snapshotter: snap.New(nil, dirpath),
		encrypter:   sc.encrypter,
		decrypter:   sc.decrypter,
	}
}

type originalSnap struct{}

func (o originalSnap) New(dirpath string) Snapshotter {
	return snap.New(nil, dirpath)
}

// OriginalSnap is the original `snap` package as an implementation of the SnapFactory interface
var OriginalSnap SnapFactory = originalSnap{}

// MigrateSnapshot reads the latest existing snapshot from one directory, encoded one way, and writes
// it to a new directory, encoded a different way
func MigrateSnapshot(oldDir, newDir string, oldFactory, newFactory SnapFactory) error {
	// use temporary snapshot directory so initialization appears atomic
	oldSnapshotter := oldFactory.New(oldDir)
	snapshot, err := oldSnapshotter.Load()
	switch err {
	case snap.ErrNoSnapshot: // if there's no snapshot, the migration succeeded
		return nil
	case nil:
		break
	default:
		return err
	}

	tmpdirpath := filepath.Clean(newDir) + ".tmp"
	if fileutil.Exist(tmpdirpath) {
		if err := os.RemoveAll(tmpdirpath); err != nil {
			return errors.Wrap(err, "could not remove temporary snapshot directory")
		}
	}
	if err := fileutil.CreateDirAll(tmpdirpath); err != nil {
		return errors.Wrap(err, "could not create temporary snapshot directory")
	}
	tmpSnapshotter := newFactory.New(tmpdirpath)

	// write the new snapshot to the temporary location
	if err = tmpSnapshotter.SaveSnap(*snapshot); err != nil {
		return err
	}

	return os.Rename(tmpdirpath, newDir)
}

// ListSnapshots lists all the snapshot files in a particular directory and returns
// the snapshot files in reverse lexical order (newest first)
func ListSnapshots(dirpath string) ([]string, error) {
	dirents, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}

	var snapshots []string
	for _, dirent := range dirents {
		if strings.HasSuffix(dirent.Name(), ".snap") {
			snapshots = append(snapshots, dirent.Name())
		}
	}

	// Sort snapshot filenames in reverse lexical order
	sort.Sort(sort.Reverse(sort.StringSlice(snapshots)))
	return snapshots, nil
}
