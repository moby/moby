package compaction

import (
	"context"
	"time"

	"github.com/moby/buildkit/util/db"
	"github.com/pkg/errors"
)

var ErrBusy = errors.New("database maintenance already requested or in progress")

type Status struct {
	Stats   db.CompactionStats
	Config  Config
	State   State
	Active  int
	Pending bool
	Manual  bool
}

// Inspect observes policy state without advancing watermarks or requesting maintenance.
func (s *Scheduler) Inspect() (Status, error) {
	s.mu.Lock()
	status := Status{Config: s.config, State: s.state, Active: s.active, Pending: s.pending, Manual: s.manual}
	s.mu.Unlock()
	var err error
	status.Stats, err = s.stats()
	return status, err
}

type Outcome struct {
	Result db.CompactResult
	Error  string
}

type Request struct {
	ctx    context.Context
	events chan string
	done   chan Outcome
}

func (r *Request) Events() <-chan string {
	return r.events
}

func (r *Request) Done() <-chan Outcome {
	return r.done
}

// Request bypasses the automatic write interval and size watermark for one
// cancellable attempt after an idle period.
// It never consumes automatic retries or forces writers to wait for completion.
func (s *Scheduler) Request(ctx context.Context) (*Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if s.stopping || context.Cause(s.ctx) != nil {
		return nil, errors.New("compaction scheduler stopped")
	}
	if s.manual || s.attempt != nil {
		return nil, ErrBusy
	}
	r := &Request{ctx: ctx, events: make(chan string, 8), done: make(chan Outcome, 1)}
	s.manual = true
	s.metrics.wait(true, true)
	s.requests <- r
	return r, nil
}

func (s *Scheduler) runRequest(r *Request, checkpoint <-chan time.Time) {
	ctx, cancel := context.WithCancelCause(r.ctx)
	stop := context.AfterFunc(s.ctx, func() { cancel(context.Cause(s.ctx)) })
	defer stop()
	defer cancel(context.Canceled)
	var outcome Outcome
	defer func() {
		s.mu.Lock()
		s.manual = false
		s.attempt = nil
		s.metrics.wait(true, false)
		s.mu.Unlock()
		r.done <- outcome
	}()
	r.events <- "waiting for idle"
	timer := time.NewTimer(checkpointInterval)
	defer timer.Stop()
	for {
		s.mu.Lock()
		delay := max(time.Until(s.lastUse.Add(s.config.IdleTimeout)), time.Millisecond)
		idle := s.active == 0 && time.Since(s.lastUse) >= s.config.IdleTimeout
		if s.active != 0 {
			delay = checkpointInterval
		}
		if idle && context.Cause(ctx) == nil {
			select {
			case capacity <- struct{}{}:
				s.attempt = cancel
				s.metrics.wait(true, false)
				writes := s.state.Writes
				s.mu.Unlock()
				result, err := s.backend.Compact(ctx, db.CompactOptions{MinReclaimBytes: s.config.MinReclaimBytes, MinReclaimPercent: s.config.MinReclaimPercent, Progress: r.events})
				cause := context.Cause(ctx)
				<-capacity
				outcome.Result = result
				if err != nil {
					outcome.Error = err.Error()
				}
				s.mu.Lock()
				now := time.Now()
				s.lastUse = now
				if result.Compacted {
					s.state.Writes -= writes
					s.state.SizeWatermark = nextWatermark(result.SizeAfter, s.config.SizeWatermark, s.config.SizeGrowthPercent)
					s.sizeDue = false
					s.lastSizeCheck = now
					s.pending = false
					s.retries = 0
					s.metrics.wait(false, false)
				}
				s.mu.Unlock()
				s.recordAttempt(s.ctx, true, result, err, cause)
				return
			default:
				delay = s.config.IdleTimeout
			}
		}
		s.mu.Unlock()
		timer.Reset(delay)
		select {
		case <-ctx.Done():
			outcome.Error = context.Cause(ctx).Error()
			return
		case <-s.wake:
		case <-timer.C:
		case <-checkpoint:
			if err := s.save(); err != nil {
				outcome.Error = err.Error()
				return
			}
		}
	}
}
