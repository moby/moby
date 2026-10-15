// Package compaction schedules database maintenance independently of the storage engine.
// Database growth is sampled at most every five minutes after committed writes,
// and reclaimability is checked after enough writes even without growth.
// Pending compaction waits for an idle period. Arriving writers
// cancel attempts up to the retry limit; subsequent attempts make writers wait.
// Changed policy state is checkpointed every five minutes and at close.
package compaction

import (
	"context"
	"math"
	"math/bits"
	"sync"
	"time"

	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/db"
	"github.com/pkg/errors"
)

const (
	checkpointInterval = 5 * time.Minute
	sizeCheckInterval  = 5 * time.Minute
)

var (
	errWriter = errors.New("compaction interrupted by a writer")
	capacity  = make(chan struct{}, 1)
)

type Config struct {
	ManualOnly             bool
	WritesPerCheck         uint64
	SizeWatermark          int64
	SizeGrowthPercent      int64
	MinReclaimBytes        int64
	IdleTimeout            time.Duration
	MaxRetry               int
	MinReclaimPercent      int64
	MinReclaimPercentFloor int64
	Metrics                *Metrics `json:"-"`
}

func DefaultConfig() Config {
	return Config{
		WritesPerCheck:         10000,
		SizeWatermark:          128 << 20,
		SizeGrowthPercent:      100,
		MinReclaimBytes:        256 << 20,
		IdleTimeout:            time.Minute,
		MaxRetry:               3,
		MinReclaimPercent:      30,
		MinReclaimPercentFloor: 10,
	}
}

func (c Config) Validate() error {
	if c.WritesPerCheck == 0 || c.SizeWatermark <= 0 || c.SizeGrowthPercent <= 0 || c.MinReclaimBytes <= 0 || c.IdleTimeout <= 0 || c.MaxRetry < 0 || c.MinReclaimPercent <= 0 || c.MinReclaimPercent > 100 || c.MinReclaimPercentFloor <= 0 || c.MinReclaimPercentFloor > c.MinReclaimPercent {
		return errors.New("invalid database compaction policy")
	}
	return nil
}

type State struct {
	Writes        uint64 `json:"writes"`
	SizeWatermark int64  `json:"sizeWatermark"`
}

type Backend interface {
	CompactionStats() (db.CompactionStats, error)
	Compact(context.Context, db.CompactOptions) (db.CompactResult, error)
	Save(State) error
}

type Scheduler struct {
	config  Config
	backend Backend
	ctx     context.Context
	stop    context.CancelCauseFunc
	wake    chan struct{}
	done    chan struct{}

	mu            sync.Mutex
	state         State
	saved         State
	active        int
	lastUse       time.Time
	lastSizeCheck time.Time
	notBefore     time.Time
	nextCheck     time.Time
	sizeDue       bool
	pending       bool
	retries       int
	attempt       context.CancelCauseFunc
	stopping      bool
	manual        bool
	requests      chan *Request
	path          string
	metrics       *databaseMetrics
}

// New starts maintenance. Call Close before closing the backend.
func New(ctx context.Context, config Config, state State, backend Backend) (*Scheduler, error) {
	return newScheduler(ctx, config, state, backend, config.Metrics.attach(""))
}

func newScheduler(ctx context.Context, config Config, state State, backend Backend, metrics *databaseMetrics) (*Scheduler, error) {
	if err := config.Validate(); err != nil {
		metrics.close()
		return nil, err
	}
	saved := state
	state.SizeWatermark = max(state.SizeWatermark, config.SizeWatermark)
	if saved == (State{}) {
		// An absent checkpoint needs no write until the policy state changes.
		saved = state
	}
	ctx, stop := context.WithCancelCause(ctx)
	s := &Scheduler{
		config:   config,
		backend:  backend,
		ctx:      ctx,
		stop:     stop,
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		state:    state,
		saved:    saved,
		lastUse:  time.Now(),
		requests: make(chan *Request, 1),
		metrics:  metrics,
	}
	go s.run()
	return s, nil
}

// Begin records access before entering the database's transaction gate.
func (s *Scheduler) Begin(write bool) {
	s.mu.Lock()
	s.active++
	if write && s.attempt != nil && (s.manual || s.retries < s.config.MaxRetry) {
		s.attempt(errWriter)
	}
	s.mu.Unlock()
}

// End counts only committed writes. It must also run after callback errors or panics.
func (s *Scheduler) End(committed bool) {
	s.mu.Lock()
	now := time.Now()
	s.active--
	idle := s.active == 0
	if idle {
		s.lastUse = now
	}
	crossed := false
	if committed {
		if s.state.Writes < math.MaxUint64 {
			s.state.Writes++
			crossed = s.state.Writes == s.config.WritesPerCheck
		}
		if !s.sizeDue && now.Sub(s.lastSizeCheck) >= sizeCheckInterval {
			s.sizeDue = true
			crossed = true
		}
	}
	wake := idle && (s.pending || s.stopping || s.manual) || crossed
	s.mu.Unlock()
	if wake {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// Stop cancels and joins maintenance before the backend is closed.
func (s *Scheduler) Stop() {
	if s.path != "" {
		files.CompareAndDelete(s.path, s)
	}
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	s.stop(errors.WithStack(context.Canceled))
	<-s.done
}

// Close checkpoints after the backend has stopped accepting transactions.
func (s *Scheduler) Close() error {
	s.Stop()
	for {
		s.mu.Lock()
		active := s.active
		s.mu.Unlock()
		if active == 0 {
			return s.save()
		}
		<-s.wake
	}
}

func (s *Scheduler) save() error {
	s.mu.Lock()
	state := s.state
	if state == s.saved {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	if err := s.backend.Save(state); err != nil {
		return err
	}
	s.mu.Lock()
	s.saved = state
	s.mu.Unlock()
	return nil
}

func (s *Scheduler) run() {
	defer close(s.done)
	defer s.metrics.close()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.manual = false
		select {
		case r := <-s.requests:
			r.done <- Outcome{Error: "compaction scheduler stopped"}
		default:
		}
	}()
	ticker := time.NewTicker(checkpointInterval)
	defer ticker.Stop()
	timer := time.NewTimer(s.config.IdleTimeout)
	defer timer.Stop()
	for {
		if context.Cause(s.ctx) != nil {
			return
		}
		s.check()
		s.mu.Lock()
		delay := checkpointInterval
		if s.pending && s.active == 0 {
			at := s.lastUse.Add(s.config.IdleTimeout)
			if s.notBefore.After(at) {
				at = s.notBefore
			}
			delay = max(time.Until(at), 0)
		} else if !s.config.ManualOnly && !s.pending && (s.state.Writes >= s.config.WritesPerCheck || s.sizeDue) {
			delay = max(time.Until(s.nextCheck), 0)
		}
		s.mu.Unlock()
		timer.Reset(delay)
		select {
		case <-s.ctx.Done():
		case <-s.wake:
		case req := <-s.requests:
			s.runRequest(req, ticker.C)
		case <-timer.C:
			s.compact()
		case <-ticker.C:
			if err := s.save(); err != nil {
				bklog.G(s.ctx).WithError(err).Warn("failed to save database compaction policy")
			}
		}
	}
}

func (s *Scheduler) check() {
	if s.config.ManualOnly {
		return
	}
	s.mu.Lock()
	now := time.Now()
	writes := s.state.Writes
	fullCheck := writes >= s.config.WritesPerCheck
	sizeCheck := s.sizeDue
	eligible := !s.pending && (fullCheck || sizeCheck) && !now.Before(s.nextCheck)
	watermark := s.state.SizeWatermark
	s.mu.Unlock()
	if !eligible {
		return
	}
	stats, err := s.stats()
	if err != nil {
		s.mu.Lock()
		s.nextCheck = now.Add(checkpointInterval)
		s.mu.Unlock()
		if !errors.Is(err, db.ErrCompactionBusy) {
			bklog.G(s.ctx).WithError(err).Warn("failed to check database compaction watermark")
		}
		return
	}
	opt := db.CompactOptions{MinReclaimBytes: s.config.MinReclaimBytes, MinReclaimPercent: s.config.MinReclaimPercent, MinReclaimPercentFloor: s.config.MinReclaimPercentFloor}
	sizeReached := stats.Size >= watermark
	writesReached := fullCheck && stats.Size >= s.config.SizeWatermark
	pending := (sizeReached || writesReached) && opt.MeetsReclaimThreshold(stats.Size, stats.Reclaimable)
	s.mu.Lock()
	s.sizeDue = false
	s.lastSizeCheck = now
	if pending {
		s.pending = true
		s.metrics.wait(false, true)
	} else if fullCheck {
		s.state.Writes -= min(s.state.Writes, writes)
	}
	s.mu.Unlock()
}

func (s *Scheduler) compact() {
	s.mu.Lock()
	if s.manual || !s.pending || s.active != 0 || time.Since(s.lastUse) < s.config.IdleTimeout || time.Now().Before(s.notBefore) || context.Cause(s.ctx) != nil {
		s.mu.Unlock()
		return
	}
	// Serialize copies across databases without making a queued writer wait for another DB.
	select {
	case capacity <- struct{}{}:
	default:
		s.notBefore = time.Now().Add(s.config.IdleTimeout)
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancelCause(s.ctx)
	s.attempt = cancel
	s.metrics.wait(false, false)
	writes := s.state.Writes
	s.mu.Unlock()
	res, err := s.backend.Compact(ctx, db.CompactOptions{MinReclaimBytes: s.config.MinReclaimBytes, MinReclaimPercent: s.config.MinReclaimPercent, MinReclaimPercentFloor: s.config.MinReclaimPercentFloor})
	cause := context.Cause(ctx)
	cancel(context.Canceled)
	<-capacity
	s.mu.Lock()
	s.attempt = nil
	now := time.Now()
	s.lastUse = now
	if res.Compacted {
		s.state.Writes -= writes
		s.state.SizeWatermark = nextWatermark(res.SizeAfter, s.config.SizeWatermark, s.config.SizeGrowthPercent)
		s.sizeDue = false
		s.lastSizeCheck = now
		s.pending = false
		s.retries = 0
	} else if errors.Is(cause, errWriter) {
		s.retries++
		s.metrics.wait(false, true)
	} else {
		// Disk pressure or a failed drain must not cause a tight retry loop.
		s.pending = false
		s.nextCheck = time.Now().Add(checkpointInterval)
	}
	s.mu.Unlock()
	s.recordAttempt(s.ctx, false, res, err, cause)
	if err != nil && (cause == nil || res.Compacted) {
		bklog.G(s.ctx).WithError(err).Warn("database compaction failed")
	}
	if res.Compacted {
		bklog.G(s.ctx).Infof("compacted database from %d to %d bytes in %s", res.SizeBefore, res.SizeAfter, res.Duration)
	}
}

func nextWatermark(size, minimum, growthPercent int64) int64 {
	if size <= 0 {
		return minimum
	}
	// Calculate size + ceil(size * growthPercent / 100), saturating instead
	// of overflowing for unusually large configured percentages.
	productHi, productLo := bits.Mul64(uint64(size), uint64(growthPercent))
	availableHi, availableLo := bits.Mul64(uint64(math.MaxInt64-size), 100)
	if productHi > availableHi || productHi == availableHi && productLo > availableLo {
		return math.MaxInt64
	}
	growth, remainder := bits.Div64(productHi, productLo, 100)
	if remainder != 0 {
		growth++
	}
	return max(minimum, size+int64(growth))
}
