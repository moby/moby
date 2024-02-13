package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/encryption"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

// This package wraps the go.etcd.io/etcd/server/v3/storage/wal package, and encrypts
// the bytes of whatever entry is passed to it, and decrypts the bytes of
// whatever entry it reads.

// WAL is the interface presented by go.etcd.io/etcd/server/v3/storage/wal.WAL that we depend upon
type WAL interface {
	ReadAll() ([]byte, raftpb.HardState, []raftpb.Entry, error)
	ReleaseLockTo(index uint64) error
	Close() error
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	SaveSnapshot(e walpb.Snapshot) error
}

// WALFactory provides an interface for the different ways to get a WAL object.
// For instance, the etcd/wal package itself provides this
type WALFactory interface {
	Create(dirpath string, metadata []byte) (WAL, error)
	Open(dirpath string, walsnap walpb.Snapshot) (WAL, error)
}

var _ WAL = &wrappedWAL{}
var _ WAL = &wal.WAL{}
var _ WALFactory = walCryptor{}

// wrappedWAL wraps a go.etcd.io/etcd/server/v3/storage/wal.WAL, and handles encrypting/decrypting
type wrappedWAL struct {
	*wal.WAL
	encrypter encryption.Encrypter
	decrypter encryption.Decrypter
}

// ReadAll wraps the wal.WAL.ReadAll() function, but it first checks to see if the
// metadata indicates that the entries are encryptd, and if so, decrypts them.
func (w *wrappedWAL) ReadAll() ([]byte, raftpb.HardState, []raftpb.Entry, error) {
	metadata, state, ents, err := w.WAL.ReadAll()
	if err != nil {
		return metadata, state, ents, err
	}
	for i, ent := range ents {
		ents[i].Data, err = encryption.Decrypt(ent.Data, w.decrypter)
		if err != nil {
			return nil, raftpb.HardState{}, nil, err
		}
	}

	return metadata, state, ents, nil
}

// Save encrypts the entry data (if an encrypter is exists) before passing it onto the
// wrapped wal.WAL's Save function.
func (w *wrappedWAL) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	var writeEnts []raftpb.Entry
	for _, ent := range ents {
		data, err := encryption.Encrypt(ent.Data, w.encrypter)
		if err != nil {
			return err
		}
		writeEnts = append(writeEnts, raftpb.Entry{
			Index: ent.Index,
			Term:  ent.Term,
			Type:  ent.Type,
			Data:  data,
		})
	}

	return w.WAL.Save(st, writeEnts)
}

// walCryptor is an object that provides the same functions as `etcd/wal`
// and `etcd/snap` that we need to open a WAL object or Snapshotter object
type walCryptor struct {
	encrypter encryption.Encrypter
	decrypter encryption.Decrypter
}

// NewWALFactory returns an object that can be used to produce objects that
// will read from and write to encrypted WALs on disk.
func NewWALFactory(encrypter encryption.Encrypter, decrypter encryption.Decrypter) WALFactory {
	return walCryptor{
		encrypter: encrypter,
		decrypter: decrypter,
	}
}

// Create returns a new WAL object with the given encrypters and decrypters.
func (wc walCryptor) Create(dirpath string, metadata []byte) (WAL, error) {
	w, err := wal.Create(nil, dirpath, metadata)
	if err != nil {
		return nil, err
	}
	return &wrappedWAL{
		WAL:       w,
		encrypter: wc.encrypter,
		decrypter: wc.decrypter,
	}, nil
}

// Open returns a new WAL object with the given encrypters and decrypters.
func (wc walCryptor) Open(dirpath string, snap walpb.Snapshot) (WAL, error) {
	w, err := wal.Open(nil, dirpath, snap)
	if err != nil {
		return nil, err
	}
	return &wrappedWAL{
		WAL:       w,
		encrypter: wc.encrypter,
		decrypter: wc.decrypter,
	}, nil
}

type originalWAL struct{}

func (o originalWAL) Create(dirpath string, metadata []byte) (WAL, error) {
	return wal.Create(nil, dirpath, metadata)
}
func (o originalWAL) Open(dirpath string, walsnap walpb.Snapshot) (WAL, error) {
	return wal.Open(nil, dirpath, walsnap)
}

// OriginalWAL is the original `wal` package as an implementation of the WALFactory interface
var OriginalWAL WALFactory = originalWAL{}

// WALData contains all the data returned by a WAL's ReadAll() function
// (metadata, hardwate, and entries)
type WALData struct {
	Metadata  []byte
	HardState raftpb.HardState
	Entries   []raftpb.Entry
}

// ReadRepairWAL opens a WAL for reading, and attempts to read it.  If we can't read it, attempts to repair
// and read again.
func ReadRepairWAL(
	ctx context.Context,
	walDir string,
	walsnap walpb.Snapshot,
	factory WALFactory,
) (WAL, WALData, error) {
	var (
		reader   WAL
		metadata []byte
		st       raftpb.HardState
		ents     []raftpb.Entry
		err      error
	)
	repaired := false
	for {
		if reader, err = factory.Open(walDir, walsnap); err != nil {
			return nil, WALData{}, errors.Wrap(err, "failed to open WAL")
		}
		if metadata, st, ents, err = reader.ReadAll(); err != nil {
			if closeErr := reader.Close(); closeErr != nil {
				return nil, WALData{}, closeErr
			}
			if _, ok := err.(encryption.ErrCannotDecrypt); ok {
				return nil, WALData{}, errors.Wrap(err, "failed to decrypt WAL")
			}
			// we can only repair ErrUnexpectedEOF and we never repair twice.
			if repaired || !errors.Is(err, io.ErrUnexpectedEOF) {
				// TODO(thaJeztah): should ReadRepairWAL be updated to handle cases where
				// some (last) of the files cannot be recovered? ("best effort" recovery?)
				// Or should an informative error be produced to help the user (which could
				// mean: remove the last file?). See TestReadRepairWAL for more details.
				return nil, WALData{}, errors.Wrap(err, "irreparable WAL error")
			}
			if !wal.Repair(nil, walDir) {
				return nil, WALData{}, errors.Wrap(err, "WAL error cannot be repaired")
			}
			log.G(ctx).WithError(err).Info("repaired WAL error")
			repaired = true
			continue
		}
		break
	}
	return reader, WALData{
		Metadata:  metadata,
		HardState: st,
		Entries:   ents,
	}, nil
}

// MigrateWALs reads existing WALs (from a particular snapshot and beyond) from one directory, encoded one way,
// and writes them to a new directory, encoded a different way
func MigrateWALs(ctx context.Context, oldDir, newDir string, oldFactory, newFactory WALFactory, snapshot walpb.Snapshot) error {
	oldReader, waldata, err := ReadRepairWAL(ctx, oldDir, snapshot, oldFactory)
	if err != nil {
		return err
	}
	oldReader.Close()

	if err := os.MkdirAll(filepath.Dir(newDir), 0o700); err != nil {
		return errors.Wrap(err, "could not create parent directory")
	}

	// keep temporary wal directory so WAL initialization appears atomic
	tmpdirpath := filepath.Clean(newDir) + ".tmp"
	if err := os.RemoveAll(tmpdirpath); err != nil {
		return errors.Wrap(err, "could not remove temporary WAL directory")
	}
	defer os.RemoveAll(tmpdirpath)

	tmpWAL, err := newFactory.Create(tmpdirpath, waldata.Metadata)
	if err != nil {
		return errors.Wrap(err, "could not create new WAL in temporary WAL directory")
	}
	defer tmpWAL.Close()

	if err := tmpWAL.SaveSnapshot(snapshot); err != nil {
		return errors.Wrap(err, "could not write WAL snapshot in temporary directory")
	}

	if err := tmpWAL.Save(waldata.HardState, waldata.Entries); err != nil {
		return errors.Wrap(err, "could not migrate WALs to temporary directory")
	}
	if err := tmpWAL.Close(); err != nil {
		return err
	}

	return os.Rename(tmpdirpath, newDir)
}

// ListWALs lists all the wals in a directory and returns the list in lexical
// order (oldest first)
func ListWALs(dirpath string) ([]string, error) {
	dirents, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}

	var wals []string
	for _, dirent := range dirents {
		if strings.HasSuffix(dirent.Name(), ".wal") {
			wals = append(wals, dirent.Name())
		}
	}

	// Sort WAL filenames in lexical order
	sort.Sort(sort.StringSlice(wals))
	return wals, nil
}
