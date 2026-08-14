package jobs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	s, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	return s, root
}

func makeJob(id, name string) *jobsv0.Job {
	return &jobsv0.Job{
		ID:   id,
		Name: name,
		Spec: &jobsv0.JobSpec{
			ContainerSpec: []byte(`{"Image":"busybox"}`),
			Trigger:       &jobsv0.Trigger{Schedule: &jobsv0.ScheduleTrigger{Cron: "0 3 * * *"}},
			Labels:        map[string]string{"com.example.k": "v"},
		},
		SpecHash:      "sha256:abc",
		State:         jobsv0.JobStateIdle,
		CreatedAtNano: 100,
		UpdatedAtNano: 100,
	}
}

func makeRun(jobID, runID, state string) *jobsv0.Run {
	return &jobsv0.Run{
		ID:            runID,
		JobID:         jobID,
		State:         state,
		CreatedAtNano: 200,
		Trigger:       &jobsv0.TriggerEvidence{Kind: jobsv0.TriggerKindManual, FiredAtNano: 200},
	}
}

func TestStoreReload(t *testing.T) {
	s, root := newTestStore(t)

	job := makeJob("job1", "backup")
	assert.NilError(t, s.CreateJob(job))
	run := makeRun("job1", "run1", jobsv0.RunStateSucceeded)
	run.ExitCode = &jobsv0.ExitCode{Value: 0}
	assert.NilError(t, s.CreateRun(run))
	assert.Check(t, is.Equal(run.Iteration, uint64(1)))

	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)

	gotJob, err := reloaded.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(gotJob, job))

	gotRun, err := reloaded.Run("job1", "run1")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(gotRun, run))

	byName, err := reloaded.JobByName("backup")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(byName.ID, "job1"))

	// The iteration counter must survive the reload: the next run continues
	// the sequence instead of reusing iteration numbers.
	next := makeRun("job1", "run2", jobsv0.RunStatePending)
	assert.NilError(t, reloaded.CreateRun(next))
	assert.Check(t, is.Equal(next.Iteration, uint64(2)))
}

func TestStoreCreateJobConflicts(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))

	err := s.CreateJob(makeJob("job1", "other"))
	assert.Check(t, cerrdefs.IsConflict(err), "duplicate ID: %v", err)

	err = s.CreateJob(makeJob("job2", "backup"))
	assert.Check(t, cerrdefs.IsConflict(err), "duplicate name: %v", err)
}

func TestStoreCreateJobInvalidID(t *testing.T) {
	s, _ := newTestStore(t)
	for _, tc := range []struct {
		doc string
		id  string
	}{
		{doc: "empty", id: ""},
		{doc: "dot", id: "."},
		{doc: "dotdot", id: ".."},
		{doc: "separator", id: "a/b"},
		{doc: "backslash", id: `a\b`},
		{doc: "too-long", id: strings.Repeat("a", 129)},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			err := s.CreateJob(makeJob(tc.id, "n-"+tc.doc))
			assert.Check(t, cerrdefs.IsInvalidArgument(err), "id %q: %v", tc.id, err)
		})
	}
}

func TestStoreUpdateJob(t *testing.T) {
	s, _ := newTestStore(t)
	job := makeJob("job1", "backup")
	assert.NilError(t, s.CreateJob(job))

	job.State = jobsv0.JobStateRunning
	job.Paused = true
	job.NextFireAtNano = 42
	assert.NilError(t, s.UpdateJob(job))
	got, err := s.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.State, jobsv0.JobStateRunning))
	assert.Check(t, got.Paused)
	assert.Check(t, is.Equal(got.NextFireAtNano, int64(42)))

	renamed := *job
	renamed.Name = "other"
	assert.Check(t, cerrdefs.IsInvalidArgument(s.UpdateJob(&renamed)))

	// A spec-hash change is rejected, and a spec mutated without changing
	// the hash is not persisted: the stored spec wins.
	respecced := *job
	respecced.SpecHash = "sha256:other"
	assert.Check(t, cerrdefs.IsInvalidArgument(s.UpdateJob(&respecced)))
	tampered, err := s.Job("job1")
	assert.NilError(t, err)
	tampered.Spec.Labels["com.example.k"] = "tampered"
	assert.NilError(t, s.UpdateJob(tampered))
	got, err = s.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Spec.Labels["com.example.k"], "v"))

	missing := makeJob("nope", "nope")
	assert.Check(t, cerrdefs.IsNotFound(s.UpdateJob(missing)))
}

func TestStoreLatestRunNotPersisted(t *testing.T) {
	s, root := newTestStore(t)
	job := makeJob("job1", "backup")
	job.LatestRun = makeRun("job1", "embedded", jobsv0.RunStateSucceeded)
	assert.NilError(t, s.CreateJob(job))

	// LatestRun is derived state: never stored, never returned.
	got, err := s.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Nil(got.LatestRun))

	got.LatestRun = makeRun("job1", "embedded2", jobsv0.RunStateFailed)
	assert.NilError(t, s.UpdateJob(got))
	got, err = s.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Nil(got.LatestRun))

	// Even a record hand-written with an embedded run loses it at load.
	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	got, err = reloaded.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Nil(got.LatestRun))
	data, err := os.ReadFile(filepath.Join(root, "job1", "job.json"))
	assert.NilError(t, err)
	assert.Check(t, !strings.Contains(string(data), "embedded"), "job.json must not embed a run snapshot: %s", data)
}

func TestStoreDeleteJobWithoutRuns(t *testing.T) {
	s, root := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))

	// With no history to keep, deletion leaves no tombstone and no
	// directory behind.
	assert.NilError(t, s.DeleteJob("job1"))
	_, err := os.Stat(filepath.Join(root, "job1"))
	assert.Check(t, errors.Is(err, fs.ErrNotExist))
	assert.Check(t, cerrdefs.IsNotFound(s.PurgeJob("job1")))

	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	_, err = reloaded.Job("job1")
	assert.Check(t, cerrdefs.IsNotFound(err))
}

func TestStoreRunLifecycle(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))

	run := makeRun("job1", "run1", jobsv0.RunStatePending)
	assert.NilError(t, s.CreateRun(run))

	assert.Check(t, cerrdefs.IsConflict(s.CreateRun(makeRun("job1", "run1", jobsv0.RunStatePending))), "duplicate run ID")
	assert.Check(t, cerrdefs.IsNotFound(s.CreateRun(makeRun("nope", "run2", jobsv0.RunStatePending))), "unknown job")

	// A non-terminal run can be updated, but its iteration is immutable.
	run.State = jobsv0.RunStateRunning
	run.ContainerID = "ctr1"
	assert.NilError(t, s.UpdateRun(run))
	tampered := *run
	tampered.Iteration = 99
	assert.Check(t, cerrdefs.IsInvalidArgument(s.UpdateRun(&tampered)))

	// A non-terminal run cannot be deleted.
	assert.Check(t, cerrdefs.IsConflict(s.DeleteRun("job1", "run1")))

	// Once terminal, a run is immutable.
	run.State = jobsv0.RunStateSucceeded
	run.ExitCode = &jobsv0.ExitCode{Value: 0}
	assert.NilError(t, s.UpdateRun(run))
	run.State = jobsv0.RunStateFailed
	assert.Check(t, cerrdefs.IsConflict(s.UpdateRun(run)))

	latest, err := s.LatestRun("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(latest.ID, "run1"))
	assert.Check(t, is.Equal(latest.State, jobsv0.RunStateSucceeded))

	assert.NilError(t, s.DeleteRun("job1", "run1"))
	_, err = s.Run("job1", "run1")
	assert.Check(t, cerrdefs.IsNotFound(err))
}

func TestStoreRunEviction(t *testing.T) {
	s, root := newTestStore(t)
	job := makeJob("job1", "backup")
	job.Spec.RunHistoryLimit = 3
	assert.NilError(t, s.CreateJob(job))

	// One active run amid terminal ones: it must survive eviction even when
	// it is the oldest record and the cap is exceeded.
	active := makeRun("job1", "run-active", jobsv0.RunStateRunning)
	assert.NilError(t, s.CreateRun(active))
	for i := range 5 {
		assert.NilError(t, s.CreateRun(makeRun("job1", fmt.Sprintf("run%d", i), jobsv0.RunStateSucceeded)))
	}

	page, _, _, err := s.Runs("job1", 10, "")
	assert.NilError(t, err)
	ids := make([]string, len(page))
	for i, r := range page {
		ids[i] = r.ID
	}
	// Cap is 3: the two newest terminal runs plus the protected active run
	// remain; the oldest terminal runs (run0..run2) were evicted first.
	assert.Check(t, is.DeepEqual(ids, []string{"run4", "run3", "run-active"}))

	// Eviction removed the records on disk too.
	_, err = os.Stat(filepath.Join(root, "job1", "runs", "run0.json"))
	assert.Check(t, errors.Is(err, fs.ErrNotExist))

	// Iteration numbering keeps counting past evicted records.
	next := makeRun("job1", "run-next", jobsv0.RunStatePending)
	assert.NilError(t, s.CreateRun(next))
	assert.Check(t, is.Equal(next.Iteration, uint64(7)))
}

func TestStoreRunsPagination(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))
	for i := range 5 {
		assert.NilError(t, s.CreateRun(makeRun("job1", fmt.Sprintf("run%d", i), jobsv0.RunStateSucceeded)))
	}

	page, cursor, stale, err := s.Runs("job1", 2, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 2))
	assert.Check(t, is.Equal(page[0].ID, "run4"))
	assert.Check(t, is.Equal(cursor, "run3"))
	assert.Check(t, !stale)

	page, cursor, stale, err = s.Runs("job1", 2, cursor)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(page[0].ID, "run2"))
	assert.Check(t, is.Equal(cursor, "run1"))
	assert.Check(t, !stale)

	// Final page: no next cursor.
	page, cursor, _, err = s.Runs("job1", 2, cursor)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(page), 1))
	assert.Check(t, is.Equal(page[0].ID, "run0"))
	assert.Check(t, is.Equal(cursor, ""))

	// A cursor naming an unknown (evicted) run restarts from the newest run
	// and reports staleness instead of failing.
	page, _, stale, err = s.Runs("job1", 2, "evicted")
	assert.NilError(t, err)
	assert.Check(t, stale)
	assert.Check(t, is.Equal(page[0].ID, "run4"))

	page, cursor, stale, err = s.Runs("job1", -1, "")
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	assert.Check(t, is.Nil(page))
	assert.Check(t, is.Equal(cursor, ""))
	assert.Check(t, !stale)
}

func TestStoreDeleteJobKeepsTombstone(t *testing.T) {
	s, root := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))
	assert.NilError(t, s.CreateRun(makeRun("job1", "run1", jobsv0.RunStateSucceeded)))

	assert.NilError(t, s.DeleteJob("job1"))
	_, err := s.Job("job1")
	assert.Check(t, cerrdefs.IsNotFound(err))

	// History outlives the job record, both in memory and across reload.
	latest, err := s.LatestRun("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(latest.ID, "run1"))
	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	_, err = reloaded.LatestRun("job1")
	assert.NilError(t, err)

	// The name is freed for reuse by a new job.
	assert.NilError(t, s.CreateJob(makeJob("job2", "backup")))

	// PurgeJob erases the tombstone.
	assert.NilError(t, s.PurgeJob("job1"))
	_, err = s.LatestRun("job1")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = os.Stat(filepath.Join(root, "job1"))
	assert.Check(t, errors.Is(err, fs.ErrNotExist))
}

func TestStoreEvictionOnTerminalTransition(t *testing.T) {
	s, _ := newTestStore(t)
	job := makeJob("job1", "backup")
	job.Spec.RunHistoryLimit = 1
	assert.NilError(t, s.CreateJob(job))

	// Two active runs hold the history over its cap of one; nothing is
	// evictable until one of them turns terminal.
	first := makeRun("job1", "run1", jobsv0.RunStateRunning)
	assert.NilError(t, s.CreateRun(first))
	assert.NilError(t, s.CreateRun(makeRun("job1", "run2", jobsv0.RunStateRunning)))

	first.State = jobsv0.RunStateSucceeded
	assert.NilError(t, s.UpdateRun(first))

	// The terminal transition triggered eviction without waiting for the
	// next CreateRun.
	_, err := s.Run("job1", "run1")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = s.Run("job1", "run2")
	assert.NilError(t, err)
}

func TestStoreReloadSortsRuns(t *testing.T) {
	s, root := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))
	// Run IDs whose lexical order inverts creation order, so the reload
	// order cannot come for free from os.ReadDir's sorted listing.
	for _, id := range []string{"z-first", "m-second", "a-third"} {
		assert.NilError(t, s.CreateRun(makeRun("job1", id, jobsv0.RunStateSucceeded)))
	}

	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)
	page, _, _, err := reloaded.Runs("job1", 10, "")
	assert.NilError(t, err)
	ids := make([]string, len(page))
	for i, r := range page {
		ids[i] = r.ID
	}
	assert.Check(t, is.DeepEqual(ids, []string{"a-third", "m-second", "z-first"}))
}

func TestStoreQuarantinesCorruptRecords(t *testing.T) {
	s, root := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("good", "good-job")))
	assert.NilError(t, s.CreateRun(makeRun("good", "run-ok", jobsv0.RunStateSucceeded)))
	assert.NilError(t, s.CreateJob(makeJob("corrupt", "corrupt-job")))
	assert.NilError(t, s.CreateJob(makeJob("future", "future-job")))
	assert.NilError(t, s.CreateJob(makeJob("mismatch", "mismatch-job")))
	assert.NilError(t, s.CreateJob(makeJob("z-dup", "good-job-dup")))

	// Damage one job record, give another a schema version from the future,
	// make one claim an ID other than its directory, give one a name
	// already taken by an earlier directory, and add three bad run records
	// to the healthy job: undecodable, filename/ID mismatch, and wrong job.
	assert.NilError(t, os.WriteFile(filepath.Join(root, "corrupt", "job.json"), []byte("{not json"), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "future", "job.json"), []byte(`{"schemaVersion":99,"job":{"ID":"future"}}`), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "mismatch", "job.json"), []byte(`{"schemaVersion":1,"job":{"ID":"other"}}`), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "z-dup", "job.json"), []byte(`{"schemaVersion":1,"job":{"ID":"z-dup","Name":"good-job"}}`), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "good", "runs", "run-bad.json"), []byte("{not json"), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "good", "runs", "run-renamed.json"), []byte(`{"schemaVersion":1,"run":{"ID":"run-other","JobID":"good"}}`), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "good", "runs", "run-stray.json"), []byte(`{"schemaVersion":1,"run":{"ID":"run-stray","JobID":"another-job"}}`), 0o600))

	reloaded, err := NewStore(t.Context(), root)
	assert.NilError(t, err)

	// The healthy job and its healthy run survive; damaged records are
	// invisible without failing the load.
	_, err = reloaded.Job("good")
	assert.NilError(t, err)
	_, err = reloaded.Run("good", "run-ok")
	assert.NilError(t, err)
	_, err = reloaded.Run("good", "run-bad")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = reloaded.Run("good", "run-other")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = reloaded.Run("good", "run-stray")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = reloaded.Job("corrupt")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = reloaded.Job("future")
	assert.Check(t, cerrdefs.IsNotFound(err))
	_, err = reloaded.Job("mismatch")
	assert.Check(t, cerrdefs.IsNotFound(err))
	// The duplicate-name record was quarantined; the first loaded holder of
	// the name (lexically earlier directory) kept it.
	_, err = reloaded.Job("z-dup")
	assert.Check(t, cerrdefs.IsNotFound(err))
	byName, err := reloaded.JobByName("good-job")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(byName.ID, "good"))

	// The damaged files were left in place for inspection.
	_, err = os.Stat(filepath.Join(root, "corrupt", "job.json"))
	assert.NilError(t, err)
}

func TestStoreReturnsCopies(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))
	assert.NilError(t, s.CreateRun(makeRun("job1", "run1", jobsv0.RunStatePending)))

	got, err := s.Job("job1")
	assert.NilError(t, err)
	got.Spec.Labels["com.example.k"] = "tampered"
	got.Spec.Trigger.Schedule.Cron = "tampered"
	got.Spec.ContainerSpec[0] = 'X'

	fresh, err := s.Job("job1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(fresh.Spec.Labels["com.example.k"], "v"))
	assert.Check(t, is.Equal(fresh.Spec.Trigger.Schedule.Cron, "0 3 * * *"))
	assert.Check(t, is.Equal(string(fresh.Spec.ContainerSpec[0]), "{"))

	run, err := s.Run("job1", "run1")
	assert.NilError(t, err)
	run.Trigger.Kind = "tampered"
	freshRun, err := s.Run("job1", "run1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(freshRun.Trigger.Kind, jobsv0.TriggerKindManual))
}

func TestStoreConcurrentRunCreation(t *testing.T) {
	s, _ := newTestStore(t)
	assert.NilError(t, s.CreateJob(makeJob("job1", "backup")))

	const workers = 16
	runs := make([]*jobsv0.Run, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			runs[i] = makeRun("job1", fmt.Sprintf("run%d", i), jobsv0.RunStateSucceeded)
			assert.Check(t, s.CreateRun(runs[i]))
		})
	}
	wg.Wait()

	// Every run got a distinct iteration in 1..workers.
	seen := make(map[uint64]bool)
	for _, run := range runs {
		assert.Check(t, run.Iteration >= 1 && run.Iteration <= workers, "iteration %d out of range", run.Iteration)
		assert.Check(t, !seen[run.Iteration], "iteration %d assigned twice", run.Iteration)
		seen[run.Iteration] = true
	}
}
