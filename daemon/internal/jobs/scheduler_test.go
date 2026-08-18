package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// fakeClock is an injectable manager clock; the scheduler is exercised by
// advancing it and calling tick directly, so no test ever sleeps on cron
// granularity.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestManagerAt builds a manager whose clock starts at the given instant.
func newTestManagerAt(t *testing.T, start time.Time) (*Manager, *fakeBackend, *fakeClock) {
	t.Helper()
	m, fake := newTestManager(t)
	clk := &fakeClock{t: start}
	m.now = clk.Now
	return m, fake, clk
}

// jan15 is an arbitrary fixed reference: 2026-01-15 00:00 UTC, a Thursday.
var jan15 = time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)

func TestSchedulerFiresAndRearms(t *testing.T) {
	m, fake, clk := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	// Registration armed the trigger and advertised the next occurrence.
	assert.Check(t, is.Equal(job.NextFireAtNano, jan15.Add(3*time.Hour).UnixNano()))

	// Before the occurrence, a tick fires nothing.
	m.sched.tick(t.Context())
	_, err = m.store.LatestRun(job.ID)
	assert.Check(t, cerrdefs.IsNotFound(err))

	// Past the occurrence, a tick fires exactly one run with schedule
	// evidence carrying the planned time.
	clk.Advance(3*time.Hour + 10*time.Second)
	m.sched.tick(t.Context())
	run, err := m.Wait(t.Context(), job.ID, "latest", jobsv0.WaitConditionRunning)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(run.Trigger.Kind, jobsv0.TriggerKindSchedule))
	assert.Check(t, is.Equal(run.Trigger.ScheduledAtNano, jan15.Add(3*time.Hour).UnixNano()))

	// While the run is in flight the record advertises no next fire.
	inFlight, err := m.Inspect(t.Context(), job.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inFlight.NextFireAtNano, int64(0)))

	fake.exit(run.ContainerID, 0, nil)
	done := waitTerminal(t, m, job.ID, run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateSucceeded))

	// After completion the job advertises the re-armed next occurrence.
	inspected, err := m.Inspect(t.Context(), job.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.NextFireAtNano, jan15.Add(27*time.Hour).UnixNano()))
}

func TestSchedulerForbidSkipsWhileRunning(t *testing.T) {
	m, fake, clk := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	clk.Advance(3*time.Hour + time.Second)
	m.sched.tick(t.Context())
	first, err := m.Wait(t.Context(), job.ID, "latest", jobsv0.WaitConditionRunning)
	assert.NilError(t, err)

	// The next occurrence comes due while the first run is still in
	// flight: forbid (the default) drops the fire. The fire is driven
	// synchronously here — tick's asynchronous dispatch of due entries is
	// covered above — so the count below cannot race the dispatch.
	clk.Advance(24 * time.Hour)
	m.fireScheduled(t.Context(), job.ID, jan15.Add(27*time.Hour))
	page, _, _, err := m.store.Runs(job.ID, 10, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 1))

	fake.exit(first.ContainerID, 0, nil)
	waitTerminal(t, m, job.ID, first.ID)
}

// shutdownCtx bounds the drain waits used mid-test.
func shutdownCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestSchedulerPauseResume(t *testing.T) {
	m, _, clk := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	assert.NilError(t, m.Pause(t.Context(), job.ID))
	paused, err := m.Inspect(t.Context(), job.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(paused.NextFireAtNano, int64(0)))

	// The occurrence passes while paused: nothing fires, and resuming
	// re-arms from the next tick without backfilling.
	clk.Advance(4 * time.Hour)
	m.sched.tick(t.Context())
	_, err = m.store.LatestRun(job.ID)
	assert.Check(t, cerrdefs.IsNotFound(err))

	assert.NilError(t, m.Resume(t.Context(), job.ID))
	resumed, err := m.Inspect(t.Context(), job.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resumed.NextFireAtNano, jan15.Add(27*time.Hour).UnixNano()))
}

func TestSchedulerMissedFiresOnStart(t *testing.T) {
	for _, tc := range []struct {
		doc           string
		policy        string
		expectCatchUp bool
	}{
		{doc: "one fires a single catch-up", policy: "", expectCatchUp: true},
		{doc: "skip drops missed fires", policy: jobsv0.MissedFiresSkip, expectCatchUp: false},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			m, fake, clk := newTestManagerAt(t, jan15)
			spec := scheduleSpec("")
			spec.Trigger.Schedule.MissedFires = tc.policy
			job, _, err := m.Create(t.Context(), "nightly", spec)
			assert.NilError(t, err)

			// The daemon was down across the 03:00 occurrence: the clock
			// advances but no tick observed it. Start applies the policy
			// from the persisted next-fire time.
			clk.Advance(5 * time.Hour)
			m.Start(t.Context())

			if tc.expectCatchUp {
				run, err := m.Wait(t.Context(), job.ID, "latest", jobsv0.WaitConditionRunning)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(run.Trigger.ScheduledAtNano, jan15.Add(3*time.Hour).UnixNano()))
				fake.exit(run.ContainerID, 0, nil)
				waitTerminal(t, m, job.ID, run.ID)
			} else {
				assert.NilError(t, m.Shutdown(shutdownCtx(t)))
				_, err := m.store.LatestRun(job.ID)
				assert.Check(t, cerrdefs.IsNotFound(err))
			}

			// Either way the schedule re-armed from the next occurrence,
			// not from the backlog.
			inspected, err := m.Inspect(t.Context(), job.ID)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(inspected.NextFireAtNano, jan15.Add(27*time.Hour).UnixNano()))
		})
	}
}

func TestSchedulerReschedule(t *testing.T) {
	m, fake, clk := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	// A manual fire with reschedule stands in for the upcoming occurrence:
	// the schedule skips 03:00 today and re-arms on tomorrow's.
	run, err := m.Run(t.Context(), job.ID, true)
	assert.NilError(t, err)
	fake.exit(run.ContainerID, 0, nil)
	waitTerminal(t, m, job.ID, run.ID)

	inspected, err := m.Inspect(t.Context(), job.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.NextFireAtNano, jan15.Add(27*time.Hour).UnixNano()))

	// Today's occurrence passes without a fire.
	clk.Advance(4 * time.Hour)
	m.sched.tick(t.Context())
	assert.NilError(t, m.Shutdown(shutdownCtx(t)))
	page, _, _, err := m.store.Runs(job.ID, 10, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 1))
}

func TestSchedulerRemoveDisarms(t *testing.T) {
	m, _, clk := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	assert.NilError(t, m.Remove(t.Context(), job.ID, ""))
	clk.Advance(4 * time.Hour)
	m.sched.tick(t.Context())
	assert.NilError(t, m.Shutdown(shutdownCtx(t)))
	_, err = m.store.LatestRun(job.ID)
	assert.Check(t, cerrdefs.IsNotFound(err), "a removed job must not fire")
}

func TestSchedulerSkipNextIsCompareAndSkip(t *testing.T) {
	m, _, _ := newTestManagerAt(t, jan15)
	job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	expected, armed := m.sched.nextFor(job.ID)
	assert.Assert(t, armed)

	// A stale expectation (the occurrence was already consumed by a tick)
	// must not advance the schedule a second time.
	_, ok := m.sched.skipNext(job.ID, expected.Add(time.Hour))
	assert.Check(t, !ok)
	current, _ := m.sched.nextFor(job.ID)
	assert.Check(t, is.Equal(current.String(), expected.String()))

	// The matching expectation advances to the following occurrence.
	next, ok := m.sched.skipNext(job.ID, expected)
	assert.Assert(t, ok)
	assert.Check(t, is.Equal(next.String(), expected.Add(24*time.Hour).String()))
}

func TestSchedulerStartWithUnparseableStoredCron(t *testing.T) {
	// A schedule the validation would never accept can still sit in the
	// store (written by a newer or buggy daemon); Start must leave the job
	// disarmed instead of failing the whole startup.
	store, _ := newTestStore(t)
	job := makeJob("job1", "broken")
	job.Spec.Trigger.Schedule.Cron = "not a cron"
	assert.NilError(t, store.CreateJob(job))

	m := NewManager(store, newFakeBackend())
	clk := &fakeClock{t: jan15}
	m.now = clk.Now
	m.Start(t.Context())
	t.Cleanup(func() { assert.Check(t, m.Shutdown(shutdownCtx(t))) })

	clk.Advance(24 * time.Hour)
	m.sched.tick(t.Context())
	_, err := m.store.LatestRun("job1")
	assert.Check(t, cerrdefs.IsNotFound(err), "a job with an unparseable schedule must never fire")
}

func TestSchedulerHonorsTimezone(t *testing.T) {
	m, _, _ := newTestManagerAt(t, jan15)
	spec := scheduleSpec("")
	spec.Trigger.Schedule.Timezone = "Europe/Paris"
	job, _, err := m.Create(t.Context(), "paris-nightly", spec)
	assert.NilError(t, err)

	// 03:00 Paris is 02:00 UTC in January.
	paris, err := time.LoadLocation("Europe/Paris")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(job.NextFireAtNano, time.Date(2026, time.January, 15, 3, 0, 0, 0, paris).UnixNano()))
	assert.Check(t, is.Equal(job.NextFireAtNano, jan15.Add(2*time.Hour).UnixNano()))
}

func TestSchedulerCreateRejectsNeverFiringCron(t *testing.T) {
	m, _ := newTestManager(t)
	spec := scheduleSpec("")
	spec.Trigger.Schedule.Cron = "0 0 30 2 *"
	_, _, err := m.Create(t.Context(), "never", spec)
	assert.Check(t, cerrdefs.IsInvalidArgument(err), "want invalid-argument, got %v", err)
}

func TestSchedulerStartStop(t *testing.T) {
	m, _, _ := newTestManagerAt(t, jan15)
	_, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)

	// The loop starts, survives a duplicate Start, and drains on Shutdown;
	// the cleanup Shutdown must stay idempotent after this one.
	m.Start(t.Context())
	m.Start(t.Context())
	assert.NilError(t, m.Shutdown(shutdownCtx(t)))
	assert.NilError(t, m.Shutdown(shutdownCtx(t)))
}
