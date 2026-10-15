package db

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

var ErrCompactionBusy = errors.New("database maintenance in progress")

// CompactOptions controls when a database is compacted. When both reclaim
// thresholds are set, satisfying either threshold permits compaction.
type CompactOptions struct {
	MinReclaimBytes   int64
	MinReclaimPercent int64
	// MinInterval also throttles failed attempts.
	MinInterval time.Duration
	// PauseTimeout bounds draining transactions. Zero selects the default.
	PauseTimeout time.Duration
	// CopyTimeout bounds copying independently of the drain. Zero uses the
	// caller's context. Filesystem calls cannot be interrupted.
	CopyTimeout time.Duration
	// Progress receives phase changes without blocking maintenance. A full channel drops updates.
	Progress chan<- string
}

// MeetsReclaimThreshold reports whether the reclaimable space satisfies at
// least one configured threshold. With no configured thresholds, it returns true.
func (o CompactOptions) MeetsReclaimThreshold(size, reclaimable int64) bool {
	if o.MinReclaimBytes <= 0 && o.MinReclaimPercent <= 0 {
		return true
	}
	if o.MinReclaimBytes > 0 && reclaimable >= o.MinReclaimBytes {
		return true
	}
	if o.MinReclaimPercent <= 0 || size <= 0 {
		return false
	}
	percent := o.MinReclaimPercent
	return reclaimable >= size/100*percent+(size%100*percent+99)/100
}

type CompactResult struct {
	// Compacted remains true if a failure occurs after replacement.
	Compacted  bool
	Reason     string
	SizeBefore int64
	SizeAfter  int64
	// Reclaimable includes both free and pending pages.
	Reclaimable int64
	Duration    time.Duration
}

type Compactor interface {
	// CompactionStats returns ErrCompactionBusy rather than waiting for maintenance.
	CompactionStats() (CompactionStats, error)
	Compact(ctx context.Context, opt CompactOptions) (CompactResult, error)
}

type CompactionStats struct {
	Size int64
	// Reclaimable includes pending pages, which become available after readers drain.
	Reclaimable int64
}

// RequiredSpace estimates the replacement file plus room for allocation overhead.
func (s CompactionStats) RequiredSpace() int64 {
	live := max(s.Size-s.Reclaimable, 0)
	return live + live/10 + 64<<20
}
