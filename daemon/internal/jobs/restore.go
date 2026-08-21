package jobs

import (
	"context"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/server/backend"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
)

// Restore reconciles state left behind by an unclean daemon stop — a crash,
// or a live-restore shutdown that deliberately detached from still-running
// containers. It runs after the daemon's own container restore and before
// Start arms schedule triggers.
//
// For each job's in-flight run (at most one, by the concurrency invariant):
//
//   - a run whose start was never recorded is failed outright: whether its
//     container was never created, created but never started, or started
//     just as the daemon died, the start evidence is lost, and guessing an
//     outcome from a container that never ran would fabricate a success.
//     Any container left behind is stopped, best-effort, so nothing keeps
//     running ownerless;
//   - a started run is re-attached to the standard exit watcher, which
//     resolves it from the container's actual state — immediately if the
//     container exited during the downtime, or whenever it exits if it is
//     still running. A timeout deadline is re-armed from the original start
//     time, and enforced immediately when it expired while the daemon was
//     down;
//   - a job stuck in the running state with a terminal (or no) run — a
//     crash between the run's terminal write and the job's idle write — is
//     returned to idle.
//
// Runs kept as tombstone history of removed jobs are not reconciled: they
// may stay in flight forever, the documented cost of retaining history past
// a job's removal.
//
// Restore must be called at most once, before Start: a second call would
// attach duplicate watchers to the re-attached runs.
func (m *Manager) Restore(ctx context.Context) {
	// Reconciliation outlives the startup call the same way watchers do.
	ctx = context.WithoutCancel(ctx)
	for _, job := range m.store.Jobs() {
		m.locks.Lock(job.ID)
		m.restoreJobLocked(ctx, job)
		m.locks.Unlock(job.ID)
	}
}

func (m *Manager) restoreJobLocked(ctx context.Context, job *jobsv0.Job) {
	run, err := m.store.LatestRun(job.ID)
	if err != nil || isTerminalRunState(run.State) {
		if job.State == jobsv0.JobStateRunning {
			job.State = jobsv0.JobStateIdle
			job.UpdatedAtNano = m.now().UnixNano()
			if err := m.store.UpdateJob(job); err != nil {
				log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID}).Warn("could not return restored job to idle")
			}
			m.broadcast(job.ID)
		}
		return
	}

	if run.StartedAtNano == 0 {
		if containerID := run.ContainerID; containerID != "" {
			// Best-effort on both counts: the stop races the completion's
			// RemoveOnFailure removal, which tolerates a still-running
			// container by keeping it.
			m.background.Go(func() {
				if err := m.backend.ContainerStop(ctx, containerID, backend.ContainerStopOptions{}); err != nil {
					log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("could not stop container of unstarted restored run")
				}
			})
		}
		m.completeRunLocked(ctx, job, run, terminalOutcome{
			state:  jobsv0.RunStateFailed,
			errMsg: "daemon restarted before the run start was recorded",
		})
		return
	}

	if job.Spec.TimeoutSeconds > 0 {
		deadline := time.Unix(0, run.StartedAtNano).Add(time.Duration(job.Spec.TimeoutSeconds) * time.Second)
		if remaining := deadline.Sub(m.now()); remaining > 0 {
			m.armTimeout(run.ID, run.ContainerID, remaining)
		} else if m.setOverride(run.ID, jobsv0.RunStateTimedOut) {
			// The deadline expired while the daemon was down; enforce it
			// now, and let the re-attached watcher record the outcome.
			containerID := run.ContainerID
			m.background.Go(func() {
				if err := m.backend.ContainerStop(ctx, containerID, backend.ContainerStopOptions{}); err != nil {
					log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID, "run": run.ID}).Warn("could not stop restored run past its deadline")
				}
			})
		}
	}
	m.background.Go(func() { m.watch(ctx, job, run) })
}
