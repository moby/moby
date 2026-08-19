package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/server/backend"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
)

// errFireQueued reports that a fire was deferred behind the current run by
// the queue concurrency policy; the deferred fire happens when that run
// completes. Callers (the scheduler) treat it as a skip, not a failure.
var errFireQueued = errors.New("fire queued behind the current run")

// errFireDropped reports that a fire was dropped because a fire is already
// queued (the queue has depth one).
var errFireDropped = errors.New("fire dropped: a fire is already queued")

// terminalOutcome is what a run ends with.
type terminalOutcome struct {
	state    string
	exitCode *int64
	errMsg   string
}

// fireManual fires a run for an explicit Run or CreateAndRun call.
func (m *Manager) fireManual(ctx context.Context, jobID string) (*jobsv0.Run, error) {
	return m.tryFire(ctx, jobID, &jobsv0.TriggerEvidence{
		Kind:        jobsv0.TriggerKindManual,
		FiredAtNano: m.now().UnixNano(),
	})
}

// tryFire is the single chokepoint every fire goes through, whatever the
// trigger: it serializes on the job's lock, applies the concurrency policy
// against the job's state re-read under that lock, and only then creates the
// run. Two triggers racing on the same job therefore cannot both pass.
func (m *Manager) tryFire(ctx context.Context, jobID string, evidence *jobsv0.TriggerEvidence) (*jobsv0.Run, error) {
	m.locks.Lock(jobID)
	defer m.locks.Unlock(jobID)

	job, err := m.store.Job(jobID)
	if err != nil {
		return nil, err
	}
	if job.State == jobsv0.JobStateRunning {
		if evidence.Kind == jobsv0.TriggerKindSchedule && scheduleConcurrency(job) == jobsv0.ConcurrencyQueue {
			m.mu.Lock()
			_, full := m.queued[jobID]
			if !full {
				m.queued[jobID] = evidence
			}
			m.mu.Unlock()
			if full {
				return nil, errFireDropped
			}
			return nil, errFireQueued
		}
		return nil, cerrdefs.ErrFailedPrecondition.WithMessage(fmt.Sprintf("job %s is already running", job.Name))
	}

	now := m.now().UnixNano()
	run := &jobsv0.Run{
		ID:            stringid.GenerateRandomID(),
		JobID:         job.ID,
		State:         jobsv0.RunStatePending,
		CreatedAtNano: now,
		Trigger:       evidence,
	}
	// The run record is persisted BEFORE the container is created. Every
	// crash-safety property depends on this ordering: a daemon crash or a
	// container-create failure leaves a discoverable run record instead of
	// an orphaned container. Do not reorder.
	if err := m.store.CreateRun(run); err != nil {
		return nil, err
	}
	job.State = jobsv0.JobStateRunning
	// A running job advertises no next fire; the completion transition
	// restores it from the scheduler's armed occurrence.
	job.NextFireAtNano = 0
	job.UpdatedAtNano = now
	if err := m.store.UpdateJob(job); err != nil {
		m.completeRunLocked(context.WithoutCancel(ctx), job, run, terminalOutcome{state: jobsv0.RunStateFailed, errMsg: "recording job state: " + err.Error()})
		return m.store.Run(job.ID, run.ID)
	}
	m.broadcast(job.ID)

	if err := m.startContainer(ctx, job, run); err != nil {
		// The failure is recorded on the run, not returned: the run exists
		// and its record is the outcome, per the API contract.
		m.completeRunLocked(context.WithoutCancel(ctx), job, run, terminalOutcome{state: jobsv0.RunStateFailed, errMsg: err.Error()})
	}
	return m.store.Run(job.ID, run.ID)
}

// startContainer creates and starts the run's container, arms the timeout,
// and hands off to the exit watcher. Called under the job's lock.
func (m *Manager) startContainer(ctx context.Context, job *jobsv0.Job, run *jobsv0.Run) error {
	req, err := decodeContainerSpec(job.Spec.ContainerSpec)
	if err != nil {
		// Validated at registration; only a corrupted store record gets here.
		return fmt.Errorf("decoding container spec: %w", err)
	}
	if req.Config.Labels == nil {
		req.Config.Labels = make(map[string]string, 2)
	}
	req.Config.Labels[LabelJobID] = job.ID
	req.Config.Labels[LabelRunID] = run.ID

	created, err := m.backend.ContainerCreate(ctx, backend.ContainerCreateConfig{
		// The run-ID suffix keeps the name unique across job generations: a
		// removed job leaves its kept containers behind, and a re-created
		// job under the same name restarts its iterations at one.
		Name:             fmt.Sprintf("job-%s-%d-%s", job.Name, run.Iteration, stringid.TruncateID(run.ID)),
		Config:           req.Config,
		HostConfig:       req.HostConfig,
		NetworkingConfig: req.NetworkingConfig,
	})
	if err != nil {
		return fmt.Errorf("creating run container: %w", err)
	}
	run.ContainerID = created.ID
	if err := m.store.UpdateRun(run); err != nil {
		return fmt.Errorf("recording run container: %w", err)
	}

	if err := m.backend.ContainerStart(ctx, created.ID, "", ""); err != nil {
		return fmt.Errorf("starting run container: %w", err)
	}
	run.State = jobsv0.RunStateRunning
	run.StartedAtNano = m.now().UnixNano()
	if err := m.store.UpdateRun(run); err != nil {
		return fmt.Errorf("recording run start: %w", err)
	}
	m.broadcast(job.ID)

	if job.Spec.TimeoutSeconds > 0 {
		m.armTimeout(run.ID, run.ContainerID, time.Duration(job.Spec.TimeoutSeconds)*time.Second)
	}
	watchCtx := context.WithoutCancel(ctx)
	m.background.Go(func() { m.watch(watchCtx, job, run) })
	return nil
}

// watch blocks on the container's final exit and records the run's outcome.
func (m *Manager) watch(ctx context.Context, job *jobsv0.Job, run *jobsv0.Run) {
	waitC, err := m.backend.ContainerWait(ctx, run.ContainerID, container.WaitConditionNotRunning)
	if err != nil {
		// The container may still be running with nobody left to observe or
		// bound it; stop it rather than leak an unwatched container.
		if stopErr := m.backend.ContainerStop(ctx, run.ContainerID, backend.ContainerStopOptions{}); stopErr != nil {
			log.G(ctx).WithError(stopErr).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("could not stop unwatchable run container")
		}
		m.locks.Lock(job.ID)
		defer m.locks.Unlock(job.ID)
		m.completeRunLocked(ctx, job, run, terminalOutcome{state: jobsv0.RunStateFailed, errMsg: "waiting on run container: " + err.Error()})
		return
	}
	var status StateStatus
	select {
	case delivered, ok := <-waitC:
		if !ok {
			// The Backend contract forbids closing without a status; treat
			// a violation as a failed observation rather than panicking on
			// a nil interface.
			m.locks.Lock(job.ID)
			defer m.locks.Unlock(job.ID)
			m.completeRunLocked(ctx, job, run, terminalOutcome{state: jobsv0.RunStateFailed, errMsg: "waiting on run container: wait channel closed without a status"})
			return
		}
		status = delivered
	case <-m.stop:
		// The manager is shutting down while the container keeps running
		// (live-restore): detach without recording. The run stays in
		// flight on disk, the exact shape restart reconciliation resolves
		// from the container's actual state on the next startup.
		return
	}

	m.locks.Lock(job.ID)
	defer m.locks.Unlock(job.ID)

	code := int64(status.ExitCode())
	outcome := terminalOutcome{exitCode: &code}
	if waitErr := status.Err(); waitErr != nil {
		outcome.errMsg = waitErr.Error()
	}
	switch override := m.peekOverride(run.ID); {
	case override != "":
		outcome.state = override
	case outcome.errMsg != "":
		outcome.state = jobsv0.RunStateFailed
	case code == 0:
		outcome.state = jobsv0.RunStateSucceeded
	default:
		outcome.state = jobsv0.RunStateFailed
	}
	m.completeRunLocked(ctx, job, run, outcome)
}

// completeRunLocked writes the run's terminal record, returns the job to
// idle, and fires any queued trigger. Called under the job's lock; job is
// the snapshot taken at fire time (its spec is immutable, its runtime state
// is re-read).
func (m *Manager) completeRunLocked(ctx context.Context, job *jobsv0.Job, run *jobsv0.Run, outcome terminalOutcome) {
	m.disarmTimeout(run.ID)
	m.clearOverride(run.ID)

	// Auto-removal happens before the terminal write: run records are
	// immutable once terminal, so ContainerGone could not be set after.
	// Timed-out and cancelled runs always keep their container for
	// postmortem inspection.
	removeContainer := (outcome.state == jobsv0.RunStateSucceeded && job.Spec.RemoveOnSuccess) ||
		(outcome.state == jobsv0.RunStateFailed && job.Spec.RemoveOnFailure)
	if removeContainer && run.ContainerID != "" {
		if err := m.backend.ContainerRm(run.ContainerID, &backend.ContainerRmConfig{}); err != nil {
			log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("keeping run container that could not be auto-removed")
		} else {
			run.ContainerGone = true
		}
	}

	run.State = outcome.state
	run.ExitCode = nil
	if outcome.exitCode != nil {
		run.ExitCode = &jobsv0.ExitCode{Value: *outcome.exitCode}
	}
	run.Error = outcome.errMsg
	run.FinishedAtNano = m.now().UnixNano()
	if err := m.store.UpdateRun(run); err != nil {
		// The job may have been purged while the run was in flight; there
		// is nothing left to record on.
		log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("could not record run outcome")
	}

	// The job may have been removed while the run was in flight; its runs
	// are then a tombstone and there is no state to restore.
	if current, err := m.store.Job(job.ID); err == nil {
		current.State = jobsv0.JobStateIdle
		if next, armed := m.sched.nextFor(job.ID); armed {
			current.NextFireAtNano = next.UnixNano()
		}
		current.UpdatedAtNano = m.now().UnixNano()
		if err := m.store.UpdateJob(current); err != nil {
			log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID}).Warn("could not return job to idle")
		}
	}
	m.broadcast(job.ID)

	m.mu.Lock()
	queued := m.queued[job.ID]
	delete(m.queued, job.ID)
	m.mu.Unlock()
	if queued != nil {
		// Fire outside the held job lock; a failure to fire is the queued
		// trigger's outcome, reported like a scheduler skip.
		m.background.Go(func() {
			if _, err := m.tryFire(ctx, job.ID, queued); err != nil {
				log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID}).Warn("queued fire was not started")
			}
		})
	}
}

// Cancel stops the job's in-flight run. It returns the in-flight run's ID,
// empty when no run was in flight; triggers keep firing. When a timeout
// claims the run's terminal decision first, the returned run is recorded
// timed_out: the run's recorded terminal state is authoritative, not this
// reply.
func (m *Manager) Cancel(ctx context.Context, jobRef string) (string, error) {
	job, err := m.resolveJob(jobRef)
	if err != nil {
		return "", err
	}
	m.locks.Lock(job.ID)
	defer m.locks.Unlock(job.ID)

	run, err := m.store.LatestRun(job.ID)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if isTerminalRunState(run.State) {
		return "", nil
	}
	if !m.setOverride(run.ID, jobsv0.RunStateCancelled) {
		// A timeout beat the cancel to the terminal decision.
		return run.ID, nil
	}
	// A pending run without a container cannot be observed here: container
	// creation happens synchronously under the job lock this method holds.
	containerID := run.ContainerID
	stopCtx := context.WithoutCancel(ctx)
	m.background.Go(func() {
		// Stopping can take the stop grace period; do not hold the job lock
		// for it. The watcher records the terminal state.
		if err := m.backend.ContainerStop(stopCtx, containerID, backend.ContainerStopOptions{}); err != nil {
			log.G(stopCtx).WithError(err).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("could not stop cancelled run container")
		}
	})
	return run.ID, nil
}

// armTimeout schedules the run's deadline: past it, the container is
// stopped and the run ends timed_out (unless already cancelled).
func (m *Manager) armTimeout(runID, containerID string, timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timers[runID] = time.AfterFunc(timeout, func() {
		// Claim the timer entry and the override in one critical section:
		// a completion racing this firing has already disarmed the entry,
		// and claiming the override past that point would leak it forever
		// (nothing clears overrides of completed runs) and stop a fresh
		// container.
		m.mu.Lock()
		if _, armed := m.timers[runID]; !armed {
			m.mu.Unlock()
			return
		}
		delete(m.timers, runID)
		if _, claimed := m.overrides[runID]; claimed {
			m.mu.Unlock()
			return
		}
		m.overrides[runID] = jobsv0.RunStateTimedOut
		m.mu.Unlock()
		if err := m.backend.ContainerStop(context.Background(), containerID, backend.ContainerStopOptions{}); err != nil {
			log.G(context.Background()).WithError(err).WithFields(log.Fields{"run": runID}).Warn("could not stop timed-out run container")
		}
	})
}

// disarmTimeout stops and forgets the run's timeout timer.
func (m *Manager) disarmTimeout(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if timer, armed := m.timers[runID]; armed {
		timer.Stop()
		delete(m.timers, runID)
	}
}

// setOverride claims the run's terminal decision; the first claimant wins.
func (m *Manager) setOverride(runID, state string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, claimed := m.overrides[runID]; claimed {
		return false
	}
	m.overrides[runID] = state
	return true
}

// peekOverride reads the run's forced terminal state without consuming it;
// clearOverride forgets it once the terminal record is written.
func (m *Manager) peekOverride(runID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.overrides[runID]
}

func (m *Manager) clearOverride(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.overrides, runID)
}

// subscribe returns a channel closed on the job's next state transition.
// Callers that return without being woken must unsubscribe, or the entry
// lingers until the job's next transition, which may never come.
func (m *Manager) subscribe(jobID string) chan struct{} {
	ch := make(chan struct{})
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notify[jobID] = append(m.notify[jobID], ch)
	return ch
}

// unsubscribe withdraws a subscription that was not woken. A channel already
// consumed by broadcast is simply no longer found.
func (m *Manager) unsubscribe(jobID string, ch chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	subs := m.notify[jobID]
	for i, sub := range subs {
		if sub == ch {
			m.notify[jobID] = slices.Delete(subs, i, i+1)
			break
		}
	}
	if len(m.notify[jobID]) == 0 {
		delete(m.notify, jobID)
	}
}

// broadcast wakes every waiter subscribed to the job.
func (m *Manager) broadcast(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.notify[jobID] {
		close(ch)
	}
	delete(m.notify, jobID)
}

// scheduleConcurrency returns the job's effective concurrency policy.
func scheduleConcurrency(job *jobsv0.Job) string {
	if job.Spec != nil && job.Spec.Trigger != nil && job.Spec.Trigger.Schedule != nil && job.Spec.Trigger.Schedule.Concurrency != "" {
		return job.Spec.Trigger.Schedule.Concurrency
	}
	return jobsv0.ConcurrencyForbid
}
