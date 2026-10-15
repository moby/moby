package boltutil

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/db"
	"github.com/moby/buildkit/util/disk"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

const (
	compactSuffix       = ".compact"
	compactTxMaxSize    = 16 << 20
	defaultPauseTimeout = 30 * time.Second
)

func compactPath(p string) string {
	return p + compactSuffix
}

type dbSize struct {
	file int64
	free int64 // Includes pending pages, reclaimable once readers have drained.
}

func (d *DB) CompactionStats() (db.CompactionStats, error) {
	if !d.hmu.TryLock() {
		return db.CompactionStats{}, db.ErrCompactionBusy
	}
	defer d.hmu.Unlock()
	if d.closed {
		return db.CompactionStats{}, errors.New("database is closed")
	}
	if d.reopenNeeded.Load() {
		return db.CompactionStats{}, errors.New("database unavailable after compaction")
	}
	sz, err := d.measure()
	return db.CompactionStats{Size: sz.file, Reclaimable: sz.free}, err
}

// Compact pauses transactions while copying live data to a replacement file.
// It requires enough free disk space to hold both files until replacement.
func (d *DB) Compact(ctx context.Context, opt db.CompactOptions) (db.CompactResult, error) {
	if !d.hmu.TryLock() {
		return db.CompactResult{Reason: db.ErrCompactionBusy.Error()}, nil
	}
	defer d.hmu.Unlock()

	if d.closed {
		return db.CompactResult{}, errors.New("database is closed")
	}
	if d.reopenNeeded.Load() {
		return db.CompactResult{}, errors.New("database unavailable after compaction")
	}
	if d.opts.ReadOnly {
		return db.CompactResult{Reason: "read-only database"}, nil
	}
	if err := context.Cause(ctx); err != nil {
		return db.CompactResult{}, err
	}
	if opt.MinInterval > 0 && !d.lastCompact.IsZero() && time.Since(d.lastCompact) < opt.MinInterval {
		return db.CompactResult{Reason: "compaction attempted recently"}, nil
	}

	before, err := d.measure()
	if err != nil {
		return db.CompactResult{}, err
	}
	res := db.CompactResult{
		SizeBefore:  before.file,
		SizeAfter:   before.file,
		Reclaimable: before.free,
	}
	if res.Reason = reclaimBelowThreshold(before, opt); res.Reason != "" {
		return res, nil
	}
	res.Reason, err = checkDiskSpace(d.path, before)
	if err != nil {
		return res, err
	}
	if res.Reason != "" {
		return res, nil
	}

	pauseTimeout := opt.PauseTimeout
	if pauseTimeout <= 0 {
		pauseTimeout = defaultPauseTimeout
	}
	pauseCtx, cancel := context.WithTimeoutCause(ctx, pauseTimeout, errors.WithStack(context.DeadlineExceeded))
	defer cancel()
	d.lastCompact = time.Now()
	select {
	case opt.Progress <- "draining transactions":
	default:
	}
	if !d.gate.pause(pauseCtx) {
		if err := context.Cause(ctx); err != nil {
			return res, err
		}
		res.Reason = "timed out waiting for in-flight transactions"
		bklog.G(ctx).Warnf("skipped compaction of %s after waiting %s for in-flight transactions", d.path, time.Since(d.lastCompact))
		return res, nil
	}
	defer d.gate.unpause()
	cancel()

	// Transactions may have changed the live size while the gate drained.
	before, err = d.measure()
	if err != nil {
		return res, err
	}
	res.SizeBefore, res.SizeAfter, res.Reclaimable = before.file, before.file, before.free
	if res.Reason = reclaimBelowThreshold(before, opt); res.Reason != "" {
		return res, nil
	}
	if res.Reason, err = checkDiskSpace(d.path, before); err != nil || res.Reason != "" {
		return res, err
	}
	start := time.Now()
	select {
	case opt.Progress <- "copying and replacing database":
	default:
	}
	if opt.CopyTimeout > 0 {
		var cancelCopy context.CancelFunc
		ctx, cancelCopy = context.WithTimeoutCause(ctx, opt.CopyTimeout, errors.WithStack(context.DeadlineExceeded))
		defer cancelCopy()
	}
	res.Compacted, err = d.replaceWithCompacted(ctx)
	res.Duration = time.Since(start)
	if res.Compacted {
		d.lastCompact = time.Now()
		fi, statErr := os.Stat(d.path)
		if statErr != nil {
			res.SizeAfter = 0
			err = stderrors.Join(err, statErr)
		} else {
			res.SizeAfter = fi.Size()
		}
	}
	return res, err
}

// Do not start a write to refresh statistics: it can block outside the pause timeout.
func (d *DB) measure() (dbSize, error) {
	fi, err := os.Stat(d.path)
	if err != nil {
		return dbSize{}, errors.WithStack(err)
	}
	// FreePageN is initialized on open; FreeAlloc is only set after a write.
	// https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/db.go#L422-L436
	// https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/tx.go#L359-L367
	stats := d.bdb.Stats()
	free := (int64(stats.FreePageN) + int64(stats.PendingPageN)) * int64(d.pageSize)
	return dbSize{file: fi.Size(), free: free}, nil
}

func reclaimBelowThreshold(sz dbSize, opt db.CompactOptions) string {
	if !opt.MeetsReclaimThreshold(sz.file, sz.free) {
		return "reclaimable space below threshold"
	}
	return ""
}

func checkDiskSpace(p string, sz dbSize) (string, error) {
	dstat, err := disk.GetDiskStat(filepath.Dir(p))
	if err != nil {
		return "", errors.Wrap(err, "failed to stat filesystem for compaction")
	}
	if dstat.Available < (db.CompactionStats{Size: sz.file, Reclaimable: sz.free}).RequiredSpace() {
		return "insufficient free disk space for compacted copy", nil
	}
	return "", nil
}

// The caller holds hmu and has drained the transaction gate.
func (d *DB) replaceWithCompacted(ctx context.Context) (bool, error) {
	tmp := compactPath(d.path)
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, errors.Wrapf(err, "failed to remove stale compaction file %s", tmp)
	}
	dst, err := d.writeCompactedCopy(ctx, tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	defer func() {
		if d.bdb != dst {
			_ = dst.Close()
		}
		_ = os.Remove(tmp)
	}()
	if err := context.Cause(ctx); err != nil {
		return false, err
	}
	return d.installCompacted(dst)
}

func (d *DB) writeCompactedCopy(ctx context.Context, tmp string) (*bolt.DB, error) {
	fi, err := os.Stat(d.path) // #nosec G703 -- Database paths are daemon configuration, not build input.
	if err != nil {
		return nil, errors.WithStack(err)
	}
	opts := *d.opts
	opts.NoSync = true // Sync the completed copy instead of every intermediate commit.
	opts.PageSize = d.pageSize
	opts.OpenFile = func(name string, flags int, mode os.FileMode) (*os.File, error) {
		return os.OpenFile(name, flags|os.O_EXCL, mode)
	}
	dst, err := bolt.Open(tmp, fi.Mode().Perm(), &opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create compaction file %s", tmp)
	}
	if err := compact(ctx, dst, d.bdb); err != nil {
		_ = dst.Close()
		return nil, errors.Wrap(err, "failed to copy database")
	}
	// Explicit Sync flushes the copy even with NoSync enabled.
	// https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/db.go#L1095-L1108
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return nil, errors.Wrap(err, "failed to sync compacted copy")
	}
	dst.NoSync = d.opts.NoSync
	return dst, nil
}

// Adapted from bbolt v1.5.0's Compact, with cancellation between records.
// https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/compact.go#L8
func compact(ctx context.Context, dst, src *bolt.DB) error {
	tx, err := dst.Begin(true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var size int64
	put := func(path [][]byte, k, v []byte, sequence uint64) error {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		n := int64(len(k)) + int64(len(v))
		if size+n > compactTxMaxSize && size > 0 {
			if err := tx.Commit(); err != nil {
				return err
			}
			next, err := dst.Begin(true)
			if err != nil {
				return err
			}
			tx = next
			size = 0
		}
		size += n
		if len(path) == 0 {
			b, err := tx.CreateBucket(k)
			if err != nil {
				return err
			}
			return b.SetSequence(sequence)
		}
		b := tx.Bucket(path[0])
		for _, name := range path[1:] {
			b = b.Bucket(name)
		}
		b.FillPercent = 1
		if v != nil {
			return b.Put(k, v)
		}
		nested, err := b.CreateBucket(k)
		if err != nil {
			return err
		}
		return nested.SetSequence(sequence)
	}

	var walk func(*bolt.Bucket, [][]byte) error
	walk = func(b *bolt.Bucket, path [][]byte) error {
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				return put(path, k, v, 0)
			}
			nested := b.Bucket(k)
			if err := put(path, k, nil, nested.Sequence()); err != nil {
				return err
			}
			return walk(nested, append(path, k))
		})
	}
	if err := src.View(func(srcTx *bolt.Tx) error {
		return srcTx.ForEach(func(k []byte, b *bolt.Bucket) error {
			if err := put(nil, k, nil, b.Sequence()); err != nil {
				return err
			}
			return walk(b, [][]byte{k})
		})
	}); err != nil {
		return err
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return tx.Commit()
}
