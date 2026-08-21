package jobs

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/server/backend"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

// newRestartableManager builds a manager whose store root is kept so a test
// can simulate a daemon restart against the same on-disk state.
func newRestartableManager(t *testing.T) (*Manager, *fakeBackend, string) {
	t.Helper()
	root := t.TempDir()
	store, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	fake := newFakeBackend()
	m := NewManager(store, fake)
	t.Cleanup(func() { assert.Check(t, m.Shutdown(shutdownCtx(t))) })
	return m, fake, root
}

// restartManager shuts the previous manager down, reloads the store from
// disk — the path a real daemon restart takes — and reconciles.
func restartManager(t *testing.T, prev *Manager, root string, fake *fakeBackend) *Manager {
	t.Helper()
	assert.NilError(t, prev.Shutdown(shutdownCtx(t)))
	store, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	m := NewManager(store, fake)
	t.Cleanup(func() { assert.Check(t, m.Shutdown(shutdownCtx(t))) })
	m.Restore(t.Context())
	return m
}

func TestRestoreReattachesRunningContainer(t *testing.T) {
	m1, fake, root := newRestartableManager(t)
	_, _, err := m1.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m1.Run(t.Context(), "backup", false)
	assert.NilError(t, err)

	// The container is still running across the restart (live-restore).
	m2 := restartManager(t, m1, root, fake)
	kept, err := m2.InspectRun(t.Context(), "backup", run.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(kept.State, jobsv0.RunStateRunning))

	// The re-attached watcher records the exit like any other run.
	fake.exit(run.ContainerID, 0, nil)
	done := waitTerminal(t, m2, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateSucceeded))
	job, err := m2.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(job.State, jobsv0.JobStateIdle))
}

func TestRestoreResolvesContainerExitedDuringDowntime(t *testing.T) {
	m1, fake, root := newRestartableManager(t)
	_, _, err := m1.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m1.Run(t.Context(), "backup", false)
	assert.NilError(t, err)

	assert.NilError(t, m1.Shutdown(shutdownCtx(t)))
	// The container exits while the daemon is down; the record still says
	// running when the store reloads.
	fake.exit(run.ContainerID, 3, nil)

	store, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	m2 := NewManager(store, fake)
	t.Cleanup(func() { assert.Check(t, m2.Shutdown(shutdownCtx(t))) })
	m2.Restore(t.Context())

	done := waitTerminal(t, m2, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
	assert.Assert(t, done.ExitCode != nil)
	assert.Check(t, is.Equal(done.ExitCode.Value, int64(3)))
}

func TestRestoreFailsLostContainer(t *testing.T) {
	m1, fake, root := newRestartableManager(t)
	_, _, err := m1.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m1.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	_ = fake

	// The container vanished entirely while the daemon was down.
	m2 := restartManager(t, m1, root, newFakeBackend())
	done := waitTerminal(t, m2, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
	assert.Check(t, is.ErrorContains(errors.New(done.Error), "waiting on run container"))
	job, err := m2.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(job.State, jobsv0.JobStateIdle))
}

func TestRestoreFailsUnstartedRun(t *testing.T) {
	for _, tc := range []struct {
		doc           string
		withContainer bool
	}{
		{doc: "no container was created"},
		{doc: "a container was left behind", withContainer: true},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			m1, fake, root := newRestartableManager(t)
			job, _, err := m1.Create(t.Context(), "backup", manualSpec())
			assert.NilError(t, err)

			// Craft the crash shape directly in the store: a pending run
			// whose start was never recorded, on a job stuck running.
			run := &jobsv0.Run{ID: "orphan", JobID: job.ID, State: jobsv0.RunStatePending, CreatedAtNano: 1}
			if tc.withContainer {
				created, err := fake.ContainerCreate(t.Context(), backend.ContainerCreateConfig{Name: "job-backup-orphan"})
				assert.NilError(t, err)
				run.ContainerID = created.ID
			}
			assert.NilError(t, m1.store.CreateRun(run))
			stored, err := m1.store.Job(job.ID)
			assert.NilError(t, err)
			stored.State = jobsv0.JobStateRunning
			assert.NilError(t, m1.store.UpdateJob(stored))

			m2 := restartManager(t, m1, root, fake)
			done := waitStoreTerminal(t, m2, job.ID, run.ID)
			assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
			assert.Check(t, is.ErrorContains(errors.New(done.Error), "before the run start was recorded"))
			restored, err := m2.Inspect(t.Context(), job.ID)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(restored.State, jobsv0.JobStateIdle))
			if tc.withContainer {
				// The best-effort stop runs on its own goroutine, unordered
				// with the run's completion; wait for it.
				poll.WaitOn(t, func(poll.LogT) poll.Result {
					if slices.Contains(fake.stoppedContainers(), run.ContainerID) {
						return poll.Success()
					}
					return poll.Continue("container %s not stopped yet", run.ContainerID)
				}, poll.WithTimeout(10*time.Second))
			}
		})
	}
}

func TestRestoreIdlesStuckJob(t *testing.T) {
	m1, fake, root := newRestartableManager(t)
	_, _, err := m1.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m1.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	fake.exit(run.ContainerID, 0, nil)
	waitTerminal(t, m1, "backup", run.ID)

	// Crash shape: the run's terminal write landed, the job's idle write
	// did not.
	stored, err := m1.store.Job(run.JobID)
	assert.NilError(t, err)
	stored.State = jobsv0.JobStateRunning
	assert.NilError(t, m1.store.UpdateJob(stored))

	m2 := restartManager(t, m1, root, fake)
	job, err := m2.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(job.State, jobsv0.JobStateIdle))
}

func TestRestoreTimeouts(t *testing.T) {
	t.Run("expired deadline is enforced on restore", func(t *testing.T) {
		m1, fake, root := newRestartableManager(t)
		clk := &fakeClock{t: jan15}
		m1.now = clk.Now
		spec := manualSpec()
		spec.TimeoutSeconds = 60
		_, _, err := m1.Create(t.Context(), "slow", spec)
		assert.NilError(t, err)
		run, err := m1.Run(t.Context(), "slow", false)
		assert.NilError(t, err)

		assert.NilError(t, m1.Shutdown(shutdownCtx(t)))
		store, err := NewStore(t.Context(), root)
		assert.NilError(t, err)
		m2 := NewManager(store, fake)
		t.Cleanup(func() { assert.Check(t, m2.Shutdown(shutdownCtx(t))) })
		// The daemon comes back long past the run's deadline.
		clk.Advance(time.Hour)
		m2.now = clk.Now
		m2.Restore(t.Context())

		done := waitTerminal(t, m2, "slow", run.ID)
		assert.Check(t, is.Equal(done.State, jobsv0.RunStateTimedOut))
		assert.Check(t, is.Contains(fake.stoppedContainers(), run.ContainerID))
	})

	t.Run("remaining deadline is re-armed", func(t *testing.T) {
		m1, fake, root := newRestartableManager(t)
		spec := manualSpec()
		spec.TimeoutSeconds = 3600
		_, _, err := m1.Create(t.Context(), "slow", spec)
		assert.NilError(t, err)
		run, err := m1.Run(t.Context(), "slow", false)
		assert.NilError(t, err)

		m2 := restartManager(t, m1, root, fake)
		m2.mu.Lock()
		_, armed := m2.timers[run.ID]
		m2.mu.Unlock()
		assert.Check(t, armed, "the surviving deadline must be re-armed")
		fake.exit(run.ContainerID, 0, nil)
		waitTerminal(t, m2, "slow", run.ID)
	})
}
