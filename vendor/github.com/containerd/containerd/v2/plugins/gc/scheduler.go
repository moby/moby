/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/internal/tomlext"
	"github.com/containerd/containerd/v2/pkg/gc"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
)

// config configures the garbage collection policies.
type config struct {
	// PauseThreshold represents the maximum amount of time garbage
	// collection should be scheduled based on the average pause time.
	// For example, a value of 0.02 means that scheduled garbage collection
	// pauses should present at most 2% of real time,
	// or 20ms of every second.
	//
	// A maximum value of .5 is enforced to prevent over scheduling of the
	// garbage collector, trigger options are available to run in a more
	// predictable time frame after mutation.
	//
	// Default is 0.02
	PauseThreshold float64 `toml:"pause_threshold"`

	// DeletionThreshold is used to guarantee that a garbage collection is
	// scheduled after configured number of deletions have occurred
	// since the previous garbage collection. A value of 0 indicates that
	// garbage collection will not be triggered by deletion count.
	//
	// Default 0
	DeletionThreshold int `toml:"deletion_threshold"`

	// MutationThreshold is used to guarantee that a garbage collection is
	// run after a configured number of database mutations have occurred
	// since the previous garbage collection. A value of 0 indicates that
	// garbage collection will only be run after a manual trigger or
	// deletion. Unlike the deletion threshold, the mutation threshold does
	// not cause scheduling of a garbage collection, but ensures GC is run
	// at the next scheduled GC.
	//
	// Default 100
	MutationThreshold int `toml:"mutation_threshold"`

	// ScheduleDelay is the duration in the future to schedule a garbage
	// collection triggered manually or by exceeding the configured
	// threshold for deletion or mutation. A zero value will immediately
	// schedule. Use suffix "ms" for millisecond and "s" for second.
	//
	// Default is "0ms"
	ScheduleDelay tomlext.Duration `toml:"schedule_delay"`

	// StartupDelay is the delay duration to do an initial garbage
	// collection after startup. The initial garbage collection is used to
	// set the base for pause threshold and should be scheduled in the
	// future to avoid slowing down other startup processes. Use suffix
	// "ms" for millisecond and "s" for second.
	//
	// Default is "100ms"
	StartupDelay tomlext.Duration `toml:"startup_delay"`
}

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.GCPlugin,
		ID:   "scheduler",
		Requires: []plugin.Type{
			plugins.MetadataPlugin,
		},
		Config: &config{
			PauseThreshold:    0.02,
			DeletionThreshold: 0,
			MutationThreshold: 100,
			ScheduleDelay:     tomlext.FromStdTime(0),
			StartupDelay:      tomlext.FromStdTime(100 * time.Millisecond),
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			md, err := ic.GetSingle(plugins.MetadataPlugin)
			if err != nil {
				return nil, err
			}

			mdCollector, ok := md.(collector)
			if !ok {
				return nil, fmt.Errorf("%s %T must implement collector", plugins.MetadataPlugin, md)
			}

			m := newScheduler(mdCollector, ic.Config.(*config))

			ic.Meta.Exports = map[string]string{
				"PauseThreshold":    fmt.Sprint(m.pauseThreshold),
				"DeletionThreshold": fmt.Sprint(m.deletionThreshold),
				"MutationThreshold": fmt.Sprint(m.mutationThreshold),
				"ScheduleDelay":     fmt.Sprint(m.scheduleDelay),
			}

			go m.run(ic.Context)

			return m, nil
		},
	})
}

type mutationEvent struct {
	ts       time.Time
	mutation bool
	dirty    bool
}

type collector interface {
	RegisterMutationCallback(func(bool))
	GarbageCollect(context.Context) (gc.Stats, error)
}

type gcScheduler struct {
	c collector

	eventC chan mutationEvent

	waiterL sync.Mutex
	waiters []chan gc.Stats

	pauseThreshold    float64
	deletionThreshold int
	mutationThreshold int
	scheduleDelay     time.Duration
	startupDelay      time.Duration
}

func newScheduler(c collector, cfg *config) *gcScheduler {
	eventC := make(chan mutationEvent)

	s := &gcScheduler{
		c:                 c,
		eventC:            eventC,
		pauseThreshold:    cfg.PauseThreshold,
		deletionThreshold: cfg.DeletionThreshold,
		mutationThreshold: cfg.MutationThreshold,
		scheduleDelay:     time.Duration(cfg.ScheduleDelay),
		startupDelay:      time.Duration(cfg.StartupDelay),
	}

	if s.pauseThreshold < 0.0 {
		s.pauseThreshold = 0.0
	}
	if s.pauseThreshold > 0.5 {
		s.pauseThreshold = 0.5
	}
	if s.mutationThreshold < 0 {
		s.mutationThreshold = 0
	}
	if s.scheduleDelay < 0 {
		s.scheduleDelay = 0
	}
	if s.startupDelay < 0 {
		s.startupDelay = 0
	}

	c.RegisterMutationCallback(s.mutationCallback)

	return s
}

func (s *gcScheduler) ScheduleAndWait(ctx context.Context) (gc.Stats, error) {
	return s.wait(ctx, true)
}

func (s *gcScheduler) wait(ctx context.Context, trigger bool) (gc.Stats, error) {
	wc := make(chan gc.Stats, 1)
	s.waiterL.Lock()
	s.waiters = append(s.waiters, wc)
	s.waiterL.Unlock()

	if trigger {
		e := mutationEvent{
			ts: time.Now(),
		}
		go func() {
			s.eventC <- e
		}()
	}

	var gcStats gc.Stats
	select {
	case stats, ok := <-wc:
		if !ok {
			return gcStats, errors.New("gc failed")
		}
		gcStats = stats
	case <-ctx.Done():
		return gcStats, ctx.Err()
	}

	return gcStats, nil
}

func (s *gcScheduler) mutationCallback(dirty bool) {
	e := mutationEvent{
		ts:       time.Now(),
		mutation: true,
		dirty:    dirty,
	}
	go func() {
		s.eventC <- e
	}()
}

func schedule(d time.Duration) (<-chan time.Time, *time.Time) {
	next := time.Now().Add(d)
	return time.After(d), &next
}

func (s *gcScheduler) run(ctx context.Context) {
	const minimumGCTime = float64(5 * time.Millisecond)
	var (
		schedC <-chan time.Time

		lastCollection *time.Time
		nextCollection *time.Time

		interval    = time.Second
		gcTimeSum   time.Duration
		collections int

		triggered bool
		deletions int
		mutations int
	)
	if s.startupDelay > 0 {
		schedC, nextCollection = schedule(s.startupDelay)
	}
	for {
		select {
		case <-schedC:
			// Check if garbage collection can be skipped because
			// it is not needed or was not requested and reschedule
			// it to attempt again after another time interval.
			if !triggered && lastCollection != nil && deletions == 0 &&
				(s.mutationThreshold == 0 || mutations < s.mutationThreshold) {
				schedC, nextCollection = schedule(interval)
				continue
			}
		case e := <-s.eventC:
			if lastCollection != nil && lastCollection.After(e.ts) {
				continue
			}
			if e.dirty {
				deletions++
			}
			if e.mutation {
				mutations++
			} else {
				triggered = true
			}

			// Check if condition should cause immediate collection.
			if triggered ||
				(s.deletionThreshold > 0 && deletions >= s.deletionThreshold) ||
				(nextCollection == nil && ((s.deletionThreshold == 0 && deletions > 0) ||
					(s.mutationThreshold > 0 && mutations >= s.mutationThreshold))) {
				// Check if not already scheduled before delay threshold
				if nextCollection == nil || nextCollection.After(time.Now().Add(s.scheduleDelay)) {
					// TODO(dmcg): track re-schedules for tuning schedule config
					schedC, nextCollection = schedule(s.scheduleDelay)
				}
			}

			continue
		case <-ctx.Done():
			return
		}

		s.waiterL.Lock()

		stats, err := s.c.GarbageCollect(ctx)
		last := time.Now()
		if err != nil {
			log.G(ctx).WithError(err).Error("garbage collection failed")
			collectionCounter.WithValues("fail").Inc()
			var retryDelay time.Duration
			if lastCollection != nil {
				// If we have a previous collection time, reschedule based on that interval.
				retryDelay = nextCollection.Sub(*lastCollection) + time.Second
			} else {
				// If this is the first collection and it failed, use the default schedule delay.
				retryDelay = s.scheduleDelay
			}
			schedC, nextCollection = schedule(retryDelay)

			// Update last collection time even though failure occurred
			lastCollection = &last

			for _, w := range s.waiters {
				close(w)
			}
			s.waiters = nil
			s.waiterL.Unlock()
			continue
		}

		gcTime := stats.Elapsed()
		gcTimeHist.Update(gcTime)
		log.G(ctx).WithField("d", gcTime).Trace("garbage collected")
		gcTimeSum += gcTime
		collections++
		collectionCounter.WithValues("success").Inc()
		triggered = false
		deletions = 0
		mutations = 0

		// Calculate new interval with updated times
		if s.pauseThreshold > 0.0 {
			// Set interval to average gc time divided by the pause threshold
			// This algorithm ensures that a gc is scheduled to allow enough
			// runtime in between gc to reach the pause threshold.
			// Pause threshold is always 0.0 < threshold <= 0.5
			avg := float64(gcTimeSum) / float64(collections)
			// Enforce that avg is no less than minimumGCTime
			// to prevent immediate rescheduling
			if avg < minimumGCTime {
				avg = minimumGCTime
			}
			interval = time.Duration(avg/s.pauseThreshold - avg)
		}

		lastCollection = &last
		schedC, nextCollection = schedule(interval)

		for _, w := range s.waiters {
			w <- stats
		}
		s.waiters = nil
		s.waiterL.Unlock()
	}
}
