package jobs

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
)

// schedulerIdleWait bounds the loop's sleep when no job is armed, so a
// missed wake signal can only ever delay a fire, not lose it.
const schedulerIdleWait = time.Minute

// schedEntry is one armed schedule job.
type schedEntry struct {
	jobID    string
	schedule *cronSchedule
	loc      *time.Location
	next     time.Time
	index    int // position in the heap, -1 once removed
}

// schedHeap is a min-heap of armed entries ordered by next fire time.
type schedHeap []*schedEntry

func (h schedHeap) Len() int           { return len(h) }
func (h schedHeap) Less(i, j int) bool { return h[i].next.Before(h[j].next) }
func (h schedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *schedHeap) Push(x any) {
	entry := x.(*schedEntry)
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *schedHeap) Pop() any {
	old := *h
	entry := old[len(old)-1]
	old[len(old)-1] = nil
	entry.index = -1
	*h = old[:len(old)-1]
	return entry
}

// scheduler fires schedule jobs on the daemon's clock: a single goroutine
// sleeping until the earliest armed entry is due, dispatching fires through
// the manager's tryFire chokepoint like any other trigger.
//
// Lock order: the scheduler's mutex is taken while holding a job lock (Arm
// and Disarm run under the manager's transitions) but never the other way
// around — tick releases the mutex before dispatching, so fires and
// persistence take job locks lock-free of the scheduler.
type scheduler struct {
	m *Manager

	mu    sync.Mutex
	heap  schedHeap
	byJob map[string]*schedEntry

	wake chan struct{} // buffered kick after arm/disarm, so the loop re-plans
	stop chan struct{}
	done chan struct{}
}

func newScheduler(m *Manager) *scheduler {
	return &scheduler{
		m:     m,
		byJob: make(map[string]*schedEntry),
		wake:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// arm registers (or re-registers) a job's next occurrence.
func (s *scheduler) arm(jobID string, schedule *cronSchedule, loc *time.Location, next time.Time) {
	s.mu.Lock()
	if entry, armed := s.byJob[jobID]; armed {
		entry.schedule, entry.loc, entry.next = schedule, loc, next
		heap.Fix(&s.heap, entry.index)
	} else {
		entry := &schedEntry{jobID: jobID, schedule: schedule, loc: loc, next: next}
		heap.Push(&s.heap, entry)
		s.byJob[jobID] = entry
	}
	s.mu.Unlock()
	s.kick()
}

// disarm forgets a job. It is a no-op for unarmed jobs.
func (s *scheduler) disarm(jobID string) {
	s.mu.Lock()
	if entry, armed := s.byJob[jobID]; armed {
		heap.Remove(&s.heap, entry.index)
		delete(s.byJob, jobID)
	}
	s.mu.Unlock()
	s.kick()
}

// skipNext replaces a job's upcoming occurrence with the one after it, and
// reports the new occurrence. This is the reschedule semantics of the run
// API: the manual fire stands in for the upcoming scheduled one. The caller
// passes the occurrence it read before firing; when the entry has already
// moved past it — a tick consumed that occurrence while the manual run was
// starting — the skip is a no-op, so a single reschedule can never lose two
// occurrences.
func (s *scheduler) skipNext(jobID string, expected time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, armed := s.byJob[jobID]
	if !armed || !entry.next.Equal(expected) {
		return time.Time{}, false
	}
	next, ok := entry.schedule.next(entry.next, entry.loc)
	if !ok {
		heap.Remove(&s.heap, entry.index)
		delete(s.byJob, jobID)
		return time.Time{}, false
	}
	entry.next = next
	heap.Fix(&s.heap, entry.index)
	return next, true
}

// nextFor reports a job's armed occurrence.
func (s *scheduler) nextFor(jobID string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, armed := s.byJob[jobID]; armed {
		return entry.next, true
	}
	return time.Time{}, false
}

// kick nudges the loop to re-plan its sleep after the heap changed.
func (s *scheduler) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// tick fires every due entry and returns how long the loop may sleep before
// the next one. Dispatch happens after the scheduler lock is released: fires
// take job locks, which are held around arm/disarm calls into the scheduler.
func (s *scheduler) tick(ctx context.Context) time.Duration {
	now := s.m.now()

	type due struct {
		jobID       string
		scheduledAt time.Time
		next        time.Time
		hasNext     bool
	}
	var fires []due

	s.mu.Lock()
	// An entry overdue by several occurrences (a long sleep or system
	// suspend) fires once and re-arms from now: the intermediate backlog is
	// deliberately dropped, mirroring how the missed-fires policy caps the
	// catch-up at one run across a daemon restart.
	for len(s.heap) > 0 && !s.heap[0].next.After(now) {
		entry := s.heap[0]
		fire := due{jobID: entry.jobID, scheduledAt: entry.next}
		if next, ok := entry.schedule.next(now, entry.loc); ok {
			fire.next, fire.hasNext = next, true
			entry.next = next
			heap.Fix(&s.heap, entry.index)
		} else {
			// The expression has no future occurrence (e.g. a specific
			// date now past): the job stays registered but disarmed.
			heap.Remove(&s.heap, entry.index)
			delete(s.byJob, entry.jobID)
		}
		fires = append(fires, fire)
	}
	wait := schedulerIdleWait
	if len(s.heap) > 0 {
		wait = min(s.heap[0].next.Sub(now), schedulerIdleWait)
	}
	s.mu.Unlock()

	for _, fire := range fires {
		s.m.background.Go(func() { s.m.fireScheduled(ctx, fire.jobID, fire.scheduledAt) })
		var nextNano int64
		if fire.hasNext {
			nextNano = fire.next.UnixNano()
		}
		s.m.persistNextFire(fire.jobID, nextNano)
	}
	return wait
}

// run is the scheduler loop; it exits when stop is closed.
func (s *scheduler) run(ctx context.Context) {
	defer close(s.done)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
		case <-timer.C:
		}
		if !timer.Stop() {
			// Drain a concurrently fired timer so Reset arms cleanly.
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(s.tick(ctx))
	}
}

// fireScheduled routes a cron fire through the tryFire chokepoint and turns
// its refusals into skip logs: for a scheduler, a fire refused by the
// concurrency policy is normal operation, not an error.
func (m *Manager) fireScheduled(ctx context.Context, jobID string, scheduledAt time.Time) {
	evidence := &jobsv0.TriggerEvidence{
		Kind:            jobsv0.TriggerKindSchedule,
		ScheduledAtNano: scheduledAt.UnixNano(),
		FiredAtNano:     m.now().UnixNano(),
	}
	_, err := m.tryFire(ctx, jobID, evidence)
	switch {
	case err == nil:
	case errors.Is(err, errFireQueued):
		log.G(ctx).WithFields(log.Fields{"job": jobID}).Debug("scheduled fire queued behind the current run")
	case errors.Is(err, errFireDropped):
		log.G(ctx).WithFields(log.Fields{"job": jobID}).Warn("scheduled fire dropped: a fire is already queued")
	case cerrdefs.IsFailedPrecondition(err):
		log.G(ctx).WithFields(log.Fields{"job": jobID}).Warn("scheduled fire skipped: the previous run is still in flight")
	case cerrdefs.IsNotFound(err):
		// The job was removed while the fire was in flight; the removal
		// already disarmed it.
	default:
		log.G(ctx).WithError(err).WithFields(log.Fields{"job": jobID}).Warn("scheduled fire failed")
	}
}

// persistNextFire records a job's armed occurrence (zero for none) on its
// stored record, where inspect and list read it from.
func (m *Manager) persistNextFire(jobID string, atNano int64) {
	m.locks.Lock(jobID)
	defer m.locks.Unlock(jobID)
	job, err := m.store.Job(jobID)
	if err != nil {
		return // removed meanwhile; nothing to record on
	}
	if job.NextFireAtNano == atNano {
		return
	}
	// While a run is in flight the record advertises no next fire; the
	// completion transition re-reads the armed occurrence itself.
	if job.State == jobsv0.JobStateRunning && atNano != 0 {
		return
	}
	job.NextFireAtNano = atNano
	job.UpdatedAtNano = m.now().UnixNano()
	if err := m.store.UpdateJob(job); err != nil {
		log.G(context.Background()).WithError(err).WithFields(log.Fields{"job": jobID}).Warn("could not record next fire time")
	}
}
