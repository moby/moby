package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/server/backend"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// fakeStatus is a container exit observed by a fake wait.
type fakeStatus struct {
	code int
	err  error
}

func (s fakeStatus) ExitCode() int { return s.code }
func (s fakeStatus) Err() error    { return s.err }

// fakeContainer is one container tracked by the fake backend.
type fakeContainer struct {
	config  backend.ContainerCreateConfig
	started bool
	removed bool
	exited  *fakeStatus
	waiters []chan StateStatus
}

// fakeBackend implements Backend in memory. Tests drive container exits
// explicitly with exit(), and stops behave like a kill: the container exits
// with code 137.
type fakeBackend struct {
	mu         sync.Mutex
	seq        int
	containers map[string]*fakeContainer
	createErr  error
	startErr   error
	waitErr    error
	stopped    []string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{containers: make(map[string]*fakeContainer)}
}

// Ready reports the fake ready immediately: tests activate the extension
// with a live backend.
func (b *fakeBackend) Ready(context.Context) error { return nil }

func (b *fakeBackend) ContainerCreate(_ context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.createErr != nil {
		return container.CreateResponse{}, b.createErr
	}
	b.seq++
	id := fmt.Sprintf("ctr%d", b.seq)
	b.containers[id] = &fakeContainer{config: config}
	return container.CreateResponse{ID: id}, nil
}

func (b *fakeBackend) ContainerStart(_ context.Context, name string, _ string, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.startErr != nil {
		return b.startErr
	}
	ctr, exists := b.containers[name]
	if !exists {
		return errors.New("no such container")
	}
	ctr.started = true
	return nil
}

func (b *fakeBackend) ContainerStop(_ context.Context, name string, _ backend.ContainerStopOptions) error {
	b.mu.Lock()
	b.stopped = append(b.stopped, name)
	b.mu.Unlock()
	b.exit(name, 137, nil)
	return nil
}

func (b *fakeBackend) ContainerRm(name string, _ *backend.ContainerRmConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	ctr, exists := b.containers[name]
	if !exists {
		return errors.New("no such container")
	}
	ctr.removed = true
	return nil
}

func (b *fakeBackend) ContainerWait(_ context.Context, name string, _ container.WaitCondition) (<-chan StateStatus, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.waitErr != nil {
		return nil, b.waitErr
	}
	ctr, exists := b.containers[name]
	if !exists {
		return nil, errors.New("no such container")
	}
	waitC := make(chan StateStatus, 1)
	if ctr.exited != nil {
		waitC <- *ctr.exited
		return waitC, nil
	}
	ctr.waiters = append(ctr.waiters, waitC)
	return waitC, nil
}

// exit delivers a container's final exit to its waiters; later waits observe
// it immediately. A second exit for the same container is ignored.
func (b *fakeBackend) exit(name string, code int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ctr, exists := b.containers[name]
	if !exists || ctr.exited != nil {
		return
	}
	ctr.exited = &fakeStatus{code: code, err: err}
	for _, waitC := range ctr.waiters {
		waitC <- *ctr.exited
	}
	ctr.waiters = nil
}

func (b *fakeBackend) container(t *testing.T, id string) fakeContainer {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	ctr, exists := b.containers[id]
	assert.Assert(t, exists, "no container %s in fake backend", id)
	return *ctr
}

func (b *fakeBackend) createCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.containers)
}

// stoppedContainers returns a copy of the stop log, for assertions that are
// not ordered after the stop by a happens-before chain.
func (b *fakeBackend) stoppedContainers() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.stopped)
}

func newTestManager(t *testing.T) (*Manager, *fakeBackend) {
	t.Helper()
	store, _ := newTestStore(t)
	fake := newFakeBackend()
	m := NewManager(store, fake)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()
		assert.Check(t, m.Shutdown(ctx))
	})
	return m, fake
}

func manualSpec() *jobsv0.JobSpec {
	return &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","Cmd":["/backup.sh"]}`)}
}

func scheduleSpec(concurrency string) *jobsv0.JobSpec {
	return &jobsv0.JobSpec{
		ContainerSpec: []byte(`{"Image":"busybox"}`),
		Trigger: &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{
			Cron:        "0 3 * * *",
			Concurrency: concurrency,
		}},
	}
}

// waitTerminal blocks until the run reaches a terminal state.
func waitTerminal(t *testing.T, m *Manager, jobRef, runRef string) *jobsv0.Run {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	run, err := m.Wait(ctx, jobRef, runRef, jobsv0.WaitConditionTerminal)
	assert.NilError(t, err)
	return run
}

// waitStoreTerminal observes a run in the store until it turns terminal,
// for runs whose job record is gone (tombstone history) and which the
// API-level Wait therefore correctly refuses to resolve.
func waitStoreTerminal(t *testing.T, m *Manager, jobID, runID string) *jobsv0.Run {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		run, err := m.store.Run(jobID, runID)
		assert.NilError(t, err)
		if isTerminalRunState(run.State) {
			return run
		}
		sub := m.subscribe(jobID)
		select {
		case <-ctx.Done():
			t.Fatal("run never turned terminal")
		case <-sub:
		}
	}
}

func TestManagerCreateIdempotency(t *testing.T) {
	m, fake := newTestManager(t)

	job, created, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	assert.Check(t, created)

	// Same spec, different JSON formatting: same identity, no-op.
	respaced := &jobsv0.JobSpec{ContainerSpec: []byte(`{ "Cmd": ["/backup.sh"], "Image": "busybox" }`)}
	again, created, err := m.Create(t.Context(), "backup", respaced)
	assert.NilError(t, err)
	assert.Check(t, !created)
	assert.Check(t, is.Equal(again.ID, job.ID))
	assert.Check(t, is.Equal(again.SpecHash, job.SpecHash))

	// Different spec under the same name: already-exists carrying both
	// hashes, so a gRPC adapter can map it to codes.AlreadyExists.
	other := &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"alpine"}`)}
	_, _, err = m.Create(t.Context(), "backup", other)
	assert.Check(t, cerrdefs.IsAlreadyExists(err), "want already-exists, got %v", err)
	assert.Check(t, is.ErrorContains(err, job.SpecHash))

	// Registration never creates containers, whatever the trigger.
	_, _, err = m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(fake.createCount(), 0))
}

func TestManagerSpecHashCanonicalization(t *testing.T) {
	hash := func(t *testing.T, spec *jobsv0.JobSpec) string {
		t.Helper()
		m, _ := newTestManager(t)
		decoded, err := m.validateSpec(spec)
		assert.NilError(t, err)
		h, err := specHash(spec, decoded)
		assert.NilError(t, err)
		return h
	}

	base := scheduleSpec("")
	assert.Check(t, is.Equal(hash(t, base), hash(t, scheduleSpec(jobsv0.ConcurrencyForbid))),
		"an empty concurrency policy must hash like the explicit default")

	utc := scheduleSpec("")
	utc.Trigger.Schedule.Timezone = "UTC"
	assert.Check(t, is.Equal(hash(t, base), hash(t, utc)),
		"an empty timezone must hash like explicit UTC")

	capped := manualSpec()
	capped.RunHistoryLimit = DefaultRunHistoryLimit
	assert.Check(t, is.Equal(hash(t, manualSpec()), hash(t, capped)),
		"a zero history limit must hash like the explicit default")

	reordered := manualSpec()
	reordered.ContainerSpec = []byte(`{"Cmd":["/backup.sh"],"Image":"busybox"}`)
	assert.Check(t, is.Equal(hash(t, manualSpec()), hash(t, reordered)),
		"JSON key order must not change a job's identity")

	swapped := manualSpec()
	swapped.ContainerSpec = []byte(`{"Image":"busybox","Cmd":["/other.sh"]}`)
	assert.Check(t, hash(t, manualSpec()) != hash(t, swapped), "Cmd is order-sensitive identity")
}

func TestManagerValidation(t *testing.T) {
	m, _ := newTestManager(t)
	for _, tc := range []struct {
		doc  string
		name string
		spec *jobsv0.JobSpec
	}{
		{doc: "empty name", name: "", spec: manualSpec()},
		{doc: "invalid name", name: "no spaces allowed", spec: manualSpec()},
		{doc: "nil spec", name: "j", spec: nil},
		{doc: "empty container spec", name: "j", spec: &jobsv0.JobSpec{}},
		{doc: "no image", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{}`)}},
		{doc: "unknown container field", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","Imag":"typo"}`)}},
		{doc: "auto remove", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","HostConfig":{"AutoRemove":true}}`)}},
		{doc: "restart always", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","HostConfig":{"RestartPolicy":{"Name":"always"}}}`)}},
		{doc: "restart unless-stopped", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","HostConfig":{"RestartPolicy":{"Name":"unless-stopped"}}}`)}},
		{doc: "reserved job label", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Labels: map[string]string{"com.docker.job.id": "x"}}},
		{doc: "reserved container label", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","Labels":{"com.docker.job.run-id":"x"}}`)}},
		{doc: "negative timeout", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), TimeoutSeconds: -1}},
		{doc: "empty trigger declares no kind", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{}}},
		{doc: "two trigger kinds", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{Manual: true, Schedule: &jobsv0.ScheduleTrigger{Cron: "* * * * *"}}}},
		{doc: "empty cron", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{Cron: "  "}}}},
		{doc: "unknown timezone", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{Cron: "* * * * *", Timezone: "Mars/Olympus"}}}},
		{doc: "unknown concurrency", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{Cron: "* * * * *", Concurrency: "replace"}}}},
		{doc: "unknown missed fires", name: "j", spec: &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`), Trigger: &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{Cron: "* * * * *", MissedFires: "all"}}}},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			_, _, err := m.Create(t.Context(), tc.name, tc.spec)
			assert.Check(t, cerrdefs.IsInvalidArgument(err), "want invalid-argument, got %v", err)
		})
	}
}

func TestManagerRunLifecycle(t *testing.T) {
	m, fake := newTestManager(t)
	job, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)

	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(run.State, jobsv0.RunStateRunning))
	assert.Check(t, is.Equal(run.Iteration, uint64(1)))

	// The run container carries the correlation labels and the derived
	// name; the run-ID suffix keeps names unique across job generations.
	ctr := fake.container(t, run.ContainerID)
	assert.Check(t, strings.HasPrefix(ctr.config.Name, "job-backup-1-"), "container name %q", ctr.config.Name)
	assert.Check(t, is.Equal(ctr.config.Config.Labels[LabelJobID], job.ID))
	assert.Check(t, is.Equal(ctr.config.Config.Labels[LabelRunID], run.ID))
	assert.Check(t, ctr.started)

	// While running, the job reports running and a second run is refused.
	inspected, err := m.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.State, jobsv0.JobStateRunning))
	_, err = m.Run(t.Context(), "backup", false)
	assert.Check(t, cerrdefs.IsFailedPrecondition(err), "want failed-precondition, got %v", err)

	fake.exit(run.ContainerID, 0, nil)
	done := waitTerminal(t, m, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateSucceeded))
	assert.Assert(t, done.ExitCode != nil)
	assert.Check(t, is.Equal(done.ExitCode.Value, int64(0)))

	inspected, err = m.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.State, jobsv0.JobStateIdle))
	assert.Check(t, is.Equal(inspected.LatestRun.ID, run.ID))

	// The next run continues the iteration sequence.
	second, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(second.Iteration, uint64(2)))
	fake.exit(second.ContainerID, 3, nil)
	done = waitTerminal(t, m, "backup", second.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
	assert.Check(t, is.Equal(done.ExitCode.Value, int64(3)))
}

func TestManagerRunRecordsCreateFailure(t *testing.T) {
	m, fake := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	fake.createErr = errors.New("no such image: busybox")

	// The failure is an outcome, not an error: the run record exists (it is
	// written before the container is created) and carries the failure.
	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(run.State, jobsv0.RunStateFailed))
	assert.Check(t, is.ErrorContains(errors.New(run.Error), "no such image"))
	assert.Check(t, is.Nil(run.ExitCode))
	assert.Check(t, is.Equal(run.ContainerID, ""))

	job, err := m.Inspect(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(job.State, jobsv0.JobStateIdle))
}

func TestManagerConcurrencyPolicies(t *testing.T) {
	t.Run("forbid drops the fire", func(t *testing.T) {
		m, fake := newTestManager(t)
		job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
		assert.NilError(t, err)
		first, err := m.tryFire(t.Context(), job.ID, &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindSchedule})
		assert.NilError(t, err)

		_, err = m.tryFire(t.Context(), job.ID, &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindSchedule})
		assert.Check(t, cerrdefs.IsFailedPrecondition(err), "want failed-precondition, got %v", err)
		fake.exit(first.ContainerID, 0, nil)
		waitTerminal(t, m, job.ID, first.ID)
	})

	t.Run("queue defers one fire", func(t *testing.T) {
		m, fake := newTestManager(t)
		job, _, err := m.Create(t.Context(), "nightly", scheduleSpec(jobsv0.ConcurrencyQueue))
		assert.NilError(t, err)
		first, err := m.tryFire(t.Context(), job.ID, &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindSchedule})
		assert.NilError(t, err)

		_, err = m.tryFire(t.Context(), job.ID, &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindSchedule})
		assert.Check(t, is.ErrorIs(err, errFireQueued))
		_, err = m.tryFire(t.Context(), job.ID, &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindSchedule})
		assert.Check(t, is.ErrorIs(err, errFireDropped))

		// Completing the first run fires the queued one automatically. Wait
		// for the second run to be RUNNING before making it exit: its record
		// exists before its container does, so exiting on sight of the
		// record alone would race container creation.
		fake.exit(first.ContainerID, 0, nil)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		for {
			second, err := m.InspectRun(ctx, job.ID, "latest")
			assert.NilError(t, err)
			if second.Iteration == 2 && second.State == jobsv0.RunStateRunning {
				fake.exit(second.ContainerID, 0, nil)
				waitTerminal(t, m, job.ID, second.ID)
				return
			}
			sub := m.subscribe(job.ID)
			select {
			case <-ctx.Done():
				t.Fatal("queued fire never started")
			case <-sub:
			}
		}
	})
}

func TestManagerCancel(t *testing.T) {
	m, fake := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)

	// Cancelling an idle job is a no-op reporting no run.
	runID, err := m.Cancel(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(runID, ""))

	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	runID, err = m.Cancel(t.Context(), "backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(runID, run.ID))

	// The stop makes the container exit nonzero, but the cancel override
	// wins over the exit code.
	done := waitTerminal(t, m, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateCancelled))
	assert.Check(t, is.Contains(fake.stopped, run.ContainerID))
}

func TestManagerTimeout(t *testing.T) {
	m, fake := newTestManager(t)
	spec := manualSpec()
	spec.TimeoutSeconds = 1
	_, _, err := m.Create(t.Context(), "slow", spec)
	assert.NilError(t, err)

	run, err := m.Run(t.Context(), "slow", false)
	assert.NilError(t, err)

	// The container never exits by itself; the deadline stops it and the
	// timed_out override wins over the kill exit code.
	done := waitTerminal(t, m, "slow", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateTimedOut))
	assert.Check(t, is.Contains(fake.stopped, run.ContainerID))
}

func TestManagerAutoRemoval(t *testing.T) {
	m, fake := newTestManager(t)
	spec := manualSpec()
	spec.RemoveOnSuccess = true
	_, _, err := m.Create(t.Context(), "tidy", spec)
	assert.NilError(t, err)

	run, err := m.Run(t.Context(), "tidy", false)
	assert.NilError(t, err)
	fake.exit(run.ContainerID, 0, nil)
	done := waitTerminal(t, m, "tidy", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateSucceeded))
	assert.Check(t, done.ContainerGone)
	assert.Check(t, fake.container(t, run.ContainerID).removed)

	// A failed run keeps its container when only RemoveOnSuccess is set.
	second, err := m.Run(t.Context(), "tidy", false)
	assert.NilError(t, err)
	fake.exit(second.ContainerID, 1, nil)
	done = waitTerminal(t, m, "tidy", second.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
	assert.Check(t, !done.ContainerGone)
	assert.Check(t, !fake.container(t, second.ContainerID).removed)
}

func TestManagerCreateAndRunMatrix(t *testing.T) {
	newManagerWithJob := func(t *testing.T) (*Manager, *fakeBackend, *jobsv0.Job) {
		t.Helper()
		m, fake := newTestManager(t)
		job, _, err := m.Create(t.Context(), "existing", manualSpec())
		assert.NilError(t, err)
		return m, fake, job
	}
	finish := func(t *testing.T, m *Manager, fake *fakeBackend, run *jobsv0.Run) {
		t.Helper()
		fake.exit(run.ContainerID, 0, nil)
		waitTerminal(t, m, run.JobID, run.ID)
	}

	t.Run("no name no spec", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, _, err := m.CreateAndRun(t.Context(), "", nil)
		assert.Check(t, cerrdefs.IsInvalidArgument(err))
	})
	t.Run("spec only generates a name", func(t *testing.T) {
		m, fake := newTestManager(t)
		job, run, created, err := m.CreateAndRun(t.Context(), "", manualSpec())
		assert.NilError(t, err)
		assert.Check(t, created)
		assert.Check(t, job.Name != "")
		finish(t, m, fake, run)
	})
	t.Run("name only runs the existing job", func(t *testing.T) {
		m, fake, job := newManagerWithJob(t)
		got, run, created, err := m.CreateAndRun(t.Context(), "existing", nil)
		assert.NilError(t, err)
		assert.Check(t, !created)
		assert.Check(t, is.Equal(got.ID, job.ID))
		finish(t, m, fake, run)
	})
	t.Run("name only unknown job", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, _, err := m.CreateAndRun(t.Context(), "ghost", nil)
		assert.Check(t, cerrdefs.IsNotFound(err))
	})
	t.Run("name and matching spec run the existing job", func(t *testing.T) {
		m, fake, job := newManagerWithJob(t)
		got, run, created, err := m.CreateAndRun(t.Context(), "existing", manualSpec())
		assert.NilError(t, err)
		assert.Check(t, !created)
		assert.Check(t, is.Equal(got.ID, job.ID))
		finish(t, m, fake, run)
	})
	t.Run("name and different spec conflict", func(t *testing.T) {
		m, _, _ := newManagerWithJob(t)
		_, _, _, err := m.CreateAndRun(t.Context(), "existing", &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"alpine"}`)})
		assert.Check(t, cerrdefs.IsAlreadyExists(err), "want already-exists, got %v", err)
	})
	t.Run("name and spec register and run", func(t *testing.T) {
		m, fake := newTestManager(t)
		job, run, created, err := m.CreateAndRun(t.Context(), "fresh", manualSpec())
		assert.NilError(t, err)
		assert.Check(t, created)
		assert.Check(t, is.Equal(job.Name, "fresh"))
		finish(t, m, fake, run)
	})
	t.Run("schedule spec rejected", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, _, err := m.CreateAndRun(t.Context(), "nightly", scheduleSpec(""))
		assert.Check(t, cerrdefs.IsInvalidArgument(err))
	})
	t.Run("name resolving a schedule job rejected", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
		assert.NilError(t, err)
		_, _, _, err = m.CreateAndRun(t.Context(), "nightly", nil)
		assert.Check(t, cerrdefs.IsInvalidArgument(err))
	})
}

func TestManagerRemoveModes(t *testing.T) {
	setup := func(t *testing.T) (*Manager, *fakeBackend, *jobsv0.Run) {
		t.Helper()
		m, fake := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)
		run, err := m.Run(t.Context(), "backup", false)
		assert.NilError(t, err)
		fake.exit(run.ContainerID, 0, nil)
		return m, fake, waitTerminal(t, m, "backup", run.ID)
	}

	t.Run("default keeps history", func(t *testing.T) {
		m, _, run := setup(t)
		assert.NilError(t, m.Remove(t.Context(), "backup", ""))
		_, err := m.Inspect(t.Context(), "backup")
		assert.Check(t, cerrdefs.IsNotFound(err))
		kept, err := m.store.Run(run.JobID, run.ID)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(kept.State, jobsv0.RunStateSucceeded))
	})
	t.Run("remove drops history", func(t *testing.T) {
		m, _, run := setup(t)
		assert.NilError(t, m.Remove(t.Context(), "backup", jobsv0.RunsRemove))
		_, err := m.store.Run(run.JobID, run.ID)
		assert.Check(t, cerrdefs.IsNotFound(err))
	})
	t.Run("remove-finished keeps the run cancelled by the removal", func(t *testing.T) {
		m, fake, terminal := setup(t)
		inflight, err := m.Run(t.Context(), "backup", false)
		assert.NilError(t, err)

		assert.NilError(t, m.Remove(t.Context(), "backup", jobsv0.RunsRemoveFinished))
		// The already-terminal run is dropped; the run cancelled by this
		// removal was still in flight at removal time, so it is kept.
		_, err = m.store.Run(terminal.JobID, terminal.ID)
		assert.Check(t, cerrdefs.IsNotFound(err))
		kept, err := m.store.Run(inflight.JobID, inflight.ID)
		assert.NilError(t, err)
		assert.Check(t, !isTerminalRunState(kept.State) || kept.State == jobsv0.RunStateCancelled)

		// The watcher records the cancellation on the tombstone history. The
		// job record is gone, so the API-level Wait correctly refuses the
		// reference; observe the store directly instead.
		fake.exit(inflight.ContainerID, 137, nil)
		done := waitStoreTerminal(t, m, inflight.JobID, inflight.ID)
		assert.Check(t, is.Equal(done.State, jobsv0.RunStateCancelled))
	})
	t.Run("invalid mode", func(t *testing.T) {
		m, _, _ := setup(t)
		assert.Check(t, cerrdefs.IsInvalidArgument(m.Remove(t.Context(), "backup", "bogus")))
	})
}

func TestManagerPrune(t *testing.T) {
	m, fake := newTestManager(t)
	idle, _, err := m.Create(t.Context(), "idle-manual", manualSpec())
	assert.NilError(t, err)
	labelled := manualSpec()
	labelled.Labels = map[string]string{"com.example.project": "demo"}
	_, _, err = m.Create(t.Context(), "labelled-manual", labelled)
	assert.NilError(t, err)
	_, _, err = m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)
	_, _, err = m.Create(t.Context(), "busy", manualSpec())
	assert.NilError(t, err)
	busyRun, err := m.Run(t.Context(), "busy", false)
	assert.NilError(t, err)

	// Label-filtered prune touches only matching jobs.
	removed, err := m.Prune(t.Context(), []string{"com.example.project=demo"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(removed), 1))

	// An unfiltered prune removes idle manual jobs, never schedule jobs
	// (armed, not abandoned) nor running ones.
	removed, err = m.Prune(t.Context(), nil)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(removed, []string{idle.ID}))
	_, err = m.Inspect(t.Context(), "nightly")
	assert.NilError(t, err)
	_, err = m.Inspect(t.Context(), "busy")
	assert.NilError(t, err)

	fake.exit(busyRun.ContainerID, 0, nil)
	waitTerminal(t, m, "busy", busyRun.ID)
}

func TestManagerWait(t *testing.T) {
	t.Run("latest blocks until the first run", func(t *testing.T) {
		m, fake := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)

		type result struct {
			run *jobsv0.Run
			err error
		}
		waited := make(chan result, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
			defer cancel()
			run, err := m.Wait(ctx, "backup", "latest", "")
			waited <- result{run, err}
		}()

		run, err := m.Run(t.Context(), "backup", false)
		assert.NilError(t, err)
		fake.exit(run.ContainerID, 0, nil)
		got := <-waited
		assert.NilError(t, got.err)
		assert.Check(t, is.Equal(got.run.ID, run.ID))
		assert.Check(t, is.Equal(got.run.State, jobsv0.RunStateSucceeded))
	})
	t.Run("running condition satisfied by a terminal jump", func(t *testing.T) {
		m, fake := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)
		fake.createErr = errors.New("boom")
		run, err := m.Run(t.Context(), "backup", false)
		assert.NilError(t, err)

		got, err := m.Wait(t.Context(), "backup", run.ID, jobsv0.WaitConditionRunning)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(got.State, jobsv0.RunStateFailed))
	})
	t.Run("explicit unknown run", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)
		_, err = m.Wait(t.Context(), "backup", "ghost", "")
		assert.Check(t, cerrdefs.IsNotFound(err))
	})
	t.Run("invalid condition", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)
		_, err = m.Wait(t.Context(), "backup", "latest", "bogus")
		assert.Check(t, cerrdefs.IsInvalidArgument(err))
	})
	t.Run("caller detaches on context cancellation", func(t *testing.T) {
		m, _ := newTestManager(t)
		_, _, err := m.Create(t.Context(), "backup", manualSpec())
		assert.NilError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = m.Wait(ctx, "backup", "latest", "")
		assert.Check(t, is.ErrorIs(err, context.Canceled))
	})
}

func TestManagerPauseResume(t *testing.T) {
	m, _ := newTestManager(t)
	_, _, err := m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)
	_, _, err = m.Create(t.Context(), "manual", manualSpec())
	assert.NilError(t, err)

	assert.NilError(t, m.Pause(t.Context(), "nightly"))
	assert.NilError(t, m.Pause(t.Context(), "nightly")) // idempotent
	job, err := m.Inspect(t.Context(), "nightly")
	assert.NilError(t, err)
	assert.Check(t, job.Paused)

	assert.NilError(t, m.Resume(t.Context(), "nightly"))
	job, err = m.Inspect(t.Context(), "nightly")
	assert.NilError(t, err)
	assert.Check(t, !job.Paused)

	// A no-op for manual jobs, not an error.
	assert.NilError(t, m.Pause(t.Context(), "manual"))
	job, err = m.Inspect(t.Context(), "manual")
	assert.NilError(t, err)
	assert.Check(t, !job.Paused)
}

func TestManagerList(t *testing.T) {
	m, fake := newTestManager(t)
	labelled := manualSpec()
	labelled.Labels = map[string]string{"com.example.project": "demo"}
	_, _, err := m.Create(t.Context(), "alpha", labelled)
	assert.NilError(t, err)
	_, _, err = m.Create(t.Context(), "nightly", scheduleSpec(""))
	assert.NilError(t, err)
	assert.NilError(t, m.Pause(t.Context(), "nightly"))
	_, _, err = m.Create(t.Context(), "worker", manualSpec())
	assert.NilError(t, err)
	run, err := m.Run(t.Context(), "worker", false)
	assert.NilError(t, err)
	fake.exit(run.ContainerID, 2, nil)
	waitTerminal(t, m, "worker", run.ID)

	names := func(jobs []*jobsv0.Job) []string {
		out := make([]string, len(jobs))
		for i, job := range jobs {
			out[i] = job.Name
		}
		return out
	}

	all, err := m.List(t.Context(), nil)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(all), []string{"alpha", "nightly", "worker"}))

	byName, err := m.List(t.Context(), &jobsv0.ListRequest{Names: []string{"alpha", "worker"}})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(byName), []string{"alpha", "worker"}))

	byLabel, err := m.List(t.Context(), &jobsv0.ListRequest{Labels: []string{"com.example.project"}})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(byLabel), []string{"alpha"}))

	byKind, err := m.List(t.Context(), &jobsv0.ListRequest{TriggerKinds: []string{jobsv0.TriggerKindSchedule}})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(byKind), []string{"nightly"}))

	paused, err := m.List(t.Context(), &jobsv0.ListRequest{Paused: "true"})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(paused), []string{"nightly"}))

	failed, err := m.List(t.Context(), &jobsv0.ListRequest{LatestRunStates: []string{jobsv0.RunStateFailed}})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(failed), []string{"worker"}))

	_, err = m.List(t.Context(), &jobsv0.ListRequest{Paused: "maybe"})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	_, err = m.List(t.Context(), &jobsv0.ListRequest{States: []string{"bogus"}})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
}

func TestManagerGeneratedNamesRetryOnCollision(t *testing.T) {
	m, fake := newTestManager(t)

	// Exhaust a good part of the namespace cheaply: register a job, then
	// verify a nameless create-and-run picks a fresh name rather than
	// erroring on the first collision. Determinism comes from retrying, not
	// from the generator, so assert on success and name difference only.
	first, run1, _, err := m.CreateAndRun(t.Context(), "", manualSpec())
	assert.NilError(t, err)
	fake.exit(run1.ContainerID, 0, nil)
	waitTerminal(t, m, first.ID, run1.ID)

	second, run2, _, err := m.CreateAndRun(t.Context(), "", manualSpec())
	assert.NilError(t, err)
	assert.Check(t, first.Name != second.Name)
	fake.exit(run2.ContainerID, 0, nil)
	waitTerminal(t, m, second.ID, run2.ID)
}

func TestManagerReservedLabelPrefixIsExact(t *testing.T) {
	m, _ := newTestManager(t)
	// A label merely sharing the "com.docker.job" stem without the
	// reserved dot-prefix is allowed.
	spec := manualSpec()
	spec.Labels = map[string]string{"com.docker.jobs.custom": "ok"}
	_, _, err := m.Create(t.Context(), "stem", spec)
	assert.NilError(t, err)
}

func TestManagerRescheduleRequiresSchedule(t *testing.T) {
	m, _ := newTestManager(t)
	_, _, err := m.Create(t.Context(), "adhoc", manualSpec())
	assert.NilError(t, err)
	// Rebasing a cadence only means something for a schedule trigger.
	_, err = m.Run(t.Context(), "adhoc", true)
	assert.Check(t, cerrdefs.IsInvalidArgument(err), "want invalid-argument, got %v", err)
}

func TestManagerOnFailureWithRetryCapAccepted(t *testing.T) {
	m, _ := newTestManager(t)
	spec := &jobsv0.JobSpec{ContainerSpec: []byte(`{"Image":"busybox","HostConfig":{"RestartPolicy":{"Name":"on-failure","MaximumRetryCount":3}}}`)}
	_, created, err := m.Create(t.Context(), "retrying", spec)
	assert.NilError(t, err)
	assert.Check(t, created)
}

func TestManagerWatchErrorStopsContainer(t *testing.T) {
	m, fake := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	fake.waitErr = errors.New("wait channel broken")

	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)

	// The watcher cannot observe the container; it stops it rather than
	// leak it unwatched, and records the failure on the run.
	done := waitTerminal(t, m, "backup", run.ID)
	assert.Check(t, is.Equal(done.State, jobsv0.RunStateFailed))
	assert.Check(t, is.ErrorContains(errors.New(done.Error), "waiting on run container"))
	assert.Check(t, is.Contains(fake.stopped, run.ContainerID))
}

func TestManagerRunsPage(t *testing.T) {
	m, fake := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	var runs []*jobsv0.Run
	for range 3 {
		run, err := m.Run(t.Context(), "backup", false)
		assert.NilError(t, err)
		fake.exit(run.ContainerID, 0, nil)
		runs = append(runs, waitTerminal(t, m, "backup", run.ID))
	}

	page, cursor, stale, err := m.RunsPage(t.Context(), "backup", 2, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 2))
	assert.Check(t, is.Equal(page[0].ID, runs[2].ID))
	assert.Check(t, !stale)

	page, _, _, err = m.RunsPage(t.Context(), "backup", 2, cursor)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 1))
	assert.Check(t, is.Equal(page[0].ID, runs[0].ID))

	_, _, stale, err = m.RunsPage(t.Context(), "backup", 2, "evicted-cursor")
	assert.NilError(t, err)
	assert.Check(t, stale)

	page, cursor, stale, err = m.RunsPage(t.Context(), "ghost", 2, "")
	assert.Check(t, cerrdefs.IsNotFound(err))
	assert.Check(t, is.Nil(page))
	assert.Check(t, is.Equal(cursor, ""))
	assert.Check(t, !stale)
}

func TestManagerInspectRunRefs(t *testing.T) {
	m, fake := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)
	fake.exit(run.ContainerID, 0, nil)
	waitTerminal(t, m, "backup", run.ID)

	for _, tc := range []struct{ doc, ref string }{
		{doc: "empty means latest", ref: ""},
		{doc: "latest keyword", ref: "latest"},
		{doc: "explicit ID", ref: run.ID},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			got, err := m.InspectRun(t.Context(), "backup", tc.ref)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(got.ID, run.ID))
		})
	}
	_, err = m.InspectRun(t.Context(), "backup", "ghost")
	assert.Check(t, cerrdefs.IsNotFound(err))
}

func TestManagerShutdownDetachesFromRunningContainers(t *testing.T) {
	m, _ := newTestManager(t)
	_, _, err := m.Create(t.Context(), "backup", manualSpec())
	assert.NilError(t, err)
	run, err := m.Run(t.Context(), "backup", false)
	assert.NilError(t, err)

	// The container never exits — the live-restore shape, where the daemon
	// shuts down while containers deliberately keep running. Shutdown must
	// detach the watcher and return promptly instead of stalling until the
	// context expires.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	assert.NilError(t, m.Shutdown(ctx))

	// The run record is deliberately left in flight: the crash-leftover
	// shape restart reconciliation resolves on the next startup.
	kept, err := m.store.Run(run.JobID, run.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(kept.State, jobsv0.RunStateRunning))
}
