package boltutil

import (
	stderrors "errors"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/db"
	"github.com/moby/buildkit/util/db/compaction"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	bolterrors "go.etcd.io/bbolt/errors"
)

// DB gates transactions so compaction can replace the underlying handle.
type DB struct {
	path     string
	mode     fs.FileMode
	opts     *bolt.Options
	pageSize int
	fresh    bool

	gate gate

	// hmu serializes operations that close or replace the handle.
	hmu          sync.Mutex
	reopenMu     sync.Mutex //nolint:unused,nolintlint // Used only on Windows.
	reopenNeeded atomic.Bool
	bdb          *bolt.DB
	closed       bool
	lastCompact  time.Time
	policy       *compaction.Scheduler
}

var (
	_ db.DB        = (*DB)(nil)
	_ db.Compactor = (*DB)(nil)
)

func Open(p string, mode fs.FileMode, options *bolt.Options, policies ...compaction.Config) (*DB, error) {
	d, err := open(p, mode, options)
	if err != nil {
		return nil, err
	}
	return d.initialize(policies)
}

func open(p string, mode fs.FileMode, options *bolt.Options) (*DB, error) {
	if options == nil {
		options = bolt.DefaultOptions
	}
	opts := *options
	newDB := !fileHasContent(p)
	bdb, err := bolt.Open(p, mode, &opts)
	if err != nil {
		return nil, err
	}
	if !opts.ReadOnly {
		// Cleanup failure must not make SafeOpen reset a healthy database.
		if err := os.Remove(compactPath(p)); err != nil && !errors.Is(err, os.ErrNotExist) { // #nosec G703 -- Database paths are daemon configuration, not build input.
			bklog.L.WithError(err).Warnf("failed to remove stale compaction file for %s", p)
		}
	}
	return &DB{path: p, mode: mode, opts: &opts, pageSize: bdb.Info().PageSize, bdb: bdb, fresh: newDB}, nil
}

func (d *DB) initialize(policies []compaction.Config) (*DB, error) {
	if len(policies) > 1 {
		return nil, stderrors.Join(errors.New("only one compaction policy may be attached"), d.Close())
	}
	if len(policies) == 1 && !d.opts.ReadOnly {
		policy, err := compaction.NewFile(policies[0], d.path, d.fresh, d)
		if err != nil {
			return nil, stderrors.Join(err, d.Close())
		}
		d.policy = policy
	}
	return d, nil
}

func (d *DB) View(fn func(*bolt.Tx) error) error {
	if d.policy != nil {
		d.policy.Begin(false)
		defer d.policy.End(false)
	}
	if !d.gate.enter() {
		return bolterrors.ErrDatabaseNotOpen
	}
	defer d.gate.exit()
	bdb, err := d.transactionDB()
	if err != nil {
		return err
	}
	return bdb.View(fn)
}

func (d *DB) Update(fn func(*bolt.Tx) error) error {
	committed := false
	if d.policy != nil {
		d.policy.Begin(true)
		defer func() { d.policy.End(committed) }()
	}
	if !d.gate.enter() {
		return bolterrors.ErrDatabaseNotOpen
	}
	defer d.gate.exit()
	bdb, err := d.transactionDB()
	if err != nil {
		return err
	}
	err = bdb.Update(fn)
	committed = err == nil
	return err
}

func (d *DB) Close() error {
	if d.policy != nil {
		d.policy.Stop()
	}
	d.hmu.Lock()
	defer d.hmu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	d.gate.close()
	var err error
	if d.policy != nil {
		err = d.policy.Close()
	}
	if d.opts.NoFreelistSync && !d.opts.ReadOnly && !d.reopenNeeded.Load() {
		// Keep normal commits cheap, but avoid reconstructing the freelist on
		// the next open after an orderly shutdown.
		d.bdb.NoFreelistSync = false
		// Let bbolt sync the freelist pages before the new meta page.
		d.bdb.NoSync = false
		if checkpointErr := d.bdb.Update(func(*bolt.Tx) error { return nil }); checkpointErr != nil {
			bklog.L.WithError(checkpointErr).Warnf("failed to checkpoint bbolt freelist for %s", d.path)
			err = stderrors.Join(err, errors.Wrap(checkpointErr, "failed to checkpoint bbolt freelist"))
		}
	}
	return stderrors.Join(err, d.bdb.Close())
}
