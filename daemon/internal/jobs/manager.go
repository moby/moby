package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/locker"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/names"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/errdefs"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	"github.com/moby/moby/v2/internal/namesgenerator"
)

// Backend is the slice of the daemon's container backend the jobs manager
// drives runs with. The signatures match *daemon.Daemon method for method so
// the daemon satisfies the interface directly, with no adapter.
type Backend interface {
	ContainerCreate(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error
	ContainerStop(ctx context.Context, name string, options backend.ContainerStopOptions) error
	ContainerRm(name string, config *backend.ContainerRmConfig) error
	// ContainerWait with WaitConditionNotRunning resolves on the container's
	// final exit, after its restart policy is exhausted, which is exactly a
	// run's outcome.
	ContainerWait(ctx context.Context, name string, condition container.WaitCondition) (<-chan StateStatus, error)
}

// StateStatus reports a container's final exit. It mirrors the daemon
// container package's StateStatus so the daemon satisfies Backend without an
// adapter, while keeping fakes trivial to build in tests.
type StateStatus interface {
	ExitCode() int
	Err() error
}

// Reserved labels applied to every run container, correlating it with its
// job and run records across the container API.
const (
	// LabelJobID is the ID of the job that owns the container.
	LabelJobID = "com.docker.job.id"
	// LabelRunID is the ID of the run the container backs.
	LabelRunID = "com.docker.job.run-id"
)

// nameGenerationRetries bounds the attempts at generating an unused random
// job name, mirroring the daemon's own retry bound for container names.
const nameGenerationRetries = 6

// Manager owns the job lifecycle: registration with spec-hash idempotency,
// run execution through the single tryFire chokepoint, and the queries the
// extension API exposes. Persistence is delegated to the Store, container
// work to the Backend.
type Manager struct {
	store   *Store
	backend Backend
	// locks serializes state transitions per job ID: every fire, cancel,
	// removal and run-completion transition takes the job's lock, so two
	// triggers racing on the same job cannot both pass the concurrency
	// policy.
	locks *locker.Locker
	// now is the manager's clock, injectable in tests.
	now func() time.Time
	// validateCron validates a cron expression at registration time; a
	// field so tests can substitute it.
	validateCron func(expr string) error

	mu sync.Mutex
	// queued holds at most one deferred fire per job (the queue concurrency
	// policy has depth one); fired when the current run completes.
	queued map[string]*jobsv0.TriggerEvidence
	// overrides forces a run's terminal state (cancelled or timed_out)
	// ahead of its container exit; first writer wins.
	overrides map[string]string
	// timers holds the timeout timer of each in-flight run.
	timers map[string]*time.Timer
	// notify carries one-shot subscriptions per job ID, closed on every
	// job or run transition; Wait re-checks its condition on each signal.
	notify map[string][]chan struct{}
	// background tracks every goroutine the manager spawns (run watchers,
	// queued fires, container stops, the scheduler loop) so Shutdown can
	// drain them. Goroutines spawned by a tracked goroutine register before
	// their parent finishes, so the counter cannot reach zero while work is
	// still being spawned.
	background sync.WaitGroup
	// sched fires schedule jobs; its loop runs between Start and Shutdown,
	// but arming works from construction so jobs registered before Start
	// are picked up when the loop begins.
	sched   *scheduler
	started bool
	stopped bool
}

// NewManager builds a manager on top of a loaded store.
//
// It does not reconcile runs orphaned by a daemon crash: a job reloaded in
// the running state with a non-terminal run and no watcher stays stuck until
// the restart-reconciliation step (which re-attaches watchers or fails the
// run from the container's actual state) runs it. Wiring that up is the
// extension's startup responsibility, alongside Shutdown on the way down.
func NewManager(store *Store, backend Backend) *Manager {
	m := &Manager{
		store:   store,
		backend: backend,
		locks:   locker.New(),
		now:     time.Now,
		validateCron: func(expr string) error {
			_, err := parseCron(expr)
			return err
		},
		queued:    make(map[string]*jobsv0.TriggerEvidence),
		overrides: make(map[string]string),
		timers:    make(map[string]*time.Timer),
		notify:    make(map[string][]chan struct{}),
	}
	m.sched = newScheduler(m)
	return m
}

// Start arms every registered schedule job and starts the scheduler loop.
// Occurrences missed while the daemon was down — the persisted next-fire
// time is in the past — follow each job's missed-fires policy: one fires a
// single catch-up run, skip drops them; either way the schedule then re-arms
// from the next occurrence, never replaying the backlog.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	now := m.now()
	for _, job := range m.store.Jobs() {
		if triggerKind(job.Spec.Trigger) != jobsv0.TriggerKindSchedule || job.Paused {
			continue
		}
		trigger := job.Spec.Trigger.Schedule
		schedule, loc, err := parseScheduleTrigger(trigger)
		if err != nil {
			// Specs are validated at registration; only a store record from
			// a buggy or newer daemon gets here. Leave the job disarmed
			// rather than fail the whole startup.
			log.G(ctx).WithError(err).WithFields(log.Fields{"job": job.ID}).Warn("leaving job with unparseable schedule disarmed")
			// The record must not keep advertising an occurrence that will
			// never fire.
			m.persistNextFire(job.ID, 0)
			continue
		}
		// A job loaded in the running state is a crash leftover awaiting
		// run reconciliation, which the extension performs before Start in
		// the startup sequence. Left to defense here: no catch-up (the
		// orphaned run occupies the job anyway) and a zeroed next-fire so
		// the record honors the "zero while running" contract; completion
		// restores the armed occurrence.
		stale := job.State == jobsv0.JobStateRunning
		if missed := job.NextFireAtNano; !stale && missed > 0 && !time.Unix(0, missed).After(now) &&
			(trigger.MissedFires == "" || trigger.MissedFires == jobsv0.MissedFiresOne) {
			m.background.Go(func() { m.fireScheduled(ctx, job.ID, time.Unix(0, missed)) })
		}
		next, ok := schedule.next(now, loc)
		if !ok {
			m.persistNextFire(job.ID, 0)
			continue
		}
		m.sched.arm(job.ID, schedule, loc, next)
		if stale {
			m.persistNextFire(job.ID, 0)
		} else {
			m.persistNextFire(job.ID, next.UnixNano())
		}
	}
	m.background.Go(func() { m.sched.run(ctx) })
}

// parseScheduleTrigger resolves a validated schedule trigger into its parsed
// expression and timezone (empty means UTC).
func parseScheduleTrigger(trigger *jobsv0.ScheduleTrigger) (*cronSchedule, *time.Location, error) {
	schedule, err := parseCron(trigger.Cron)
	if err != nil {
		return nil, nil, err
	}
	loc := time.UTC
	if trigger.Timezone != "" {
		if loc, err = time.LoadLocation(trigger.Timezone); err != nil {
			return nil, nil, err
		}
	}
	return schedule, loc, nil
}

// Shutdown waits for in-flight run watchers to finish recording their
// outcomes, or for the context to expire. Callers must stop issuing API
// calls before invoking Shutdown: a call racing the drain could register
// new background work while the wait group is being awaited, which
// sync.WaitGroup forbids. When the context expires first, the internal
// waiter goroutine lingers until the background work eventually drains.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.started && !m.stopped {
		m.stopped = true
		close(m.sched.stop)
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() {
		m.background.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Create registers a job without starting a run. It is idempotent on the
// canonical spec hash: re-submitting an identical name+spec returns the
// existing job with created=false, and a name collision with a different
// spec is a conflict carrying both hashes.
func (m *Manager) Create(ctx context.Context, name string, spec *jobsv0.JobSpec) (_ *jobsv0.Job, created bool, _ error) {
	if err := validateJobName(name); err != nil {
		return nil, false, errdefs.InvalidParameter(err)
	}
	decoded, err := m.validateSpec(spec)
	if err != nil {
		return nil, false, err
	}
	hash, err := specHash(spec, decoded)
	if err != nil {
		return nil, false, err
	}

	// Registering a schedule job arms its trigger; the next occurrence is
	// computed up front so the returned job already advertises it, and an
	// expression with no future occurrence is rejected rather than silently
	// registering a job that would never fire.
	var (
		schedule *cronSchedule
		loc      *time.Location
		next     time.Time
	)
	if spec.Trigger != nil && spec.Trigger.Schedule != nil {
		if schedule, loc, err = parseScheduleTrigger(spec.Trigger.Schedule); err != nil {
			return nil, false, errdefs.InvalidParameter(err) // unreachable after validateSpec, kept for defense
		}
		var ok bool
		if next, ok = schedule.next(m.now(), loc); !ok {
			return nil, false, errdefs.InvalidParameter(fmt.Errorf("cron expression %q has no future occurrence", spec.Trigger.Schedule.Cron))
		}
	}

	job := &jobsv0.Job{
		ID:            stringid.GenerateRandomID(),
		Name:          name,
		Spec:          spec,
		SpecHash:      hash,
		State:         jobsv0.JobStateIdle,
		CreatedAtNano: m.now().UnixNano(),
		UpdatedAtNano: m.now().UnixNano(),
	}
	if schedule != nil {
		job.NextFireAtNano = next.UnixNano()
	}
	if err := m.store.CreateJob(job); err != nil {
		// A name conflict is idempotent when the stored spec matches;
		// re-read under the store's authority rather than trusting a
		// pre-flight lookup, so concurrent creates converge.
		if cerrdefs.IsConflict(err) {
			existing, lookupErr := m.store.JobByName(name)
			if lookupErr != nil {
				return nil, false, err
			}
			if existing.SpecHash == hash {
				return m.composeJob(existing), false, nil
			}
			return nil, false, cerrdefs.ErrAlreadyExists.WithMessage(fmt.Sprintf("job name %s is already in use by a different spec (stored %s, submitted %s)", name, existing.SpecHash, hash))
		}
		return nil, false, err
	}
	if schedule != nil {
		m.sched.arm(job.ID, schedule, loc, next)
	}
	return m.composeJob(job), true, nil
}

// CreateAndRun atomically resolves or registers a manual job and fires a
// run, implementing the name/spec resolution matrix of the API contract.
func (m *Manager) CreateAndRun(ctx context.Context, name string, spec *jobsv0.JobSpec) (_ *jobsv0.Job, _ *jobsv0.Run, created bool, _ error) {
	if name == "" && spec == nil {
		return nil, nil, false, errdefs.InvalidParameter(errors.New("at least one of name and spec is required"))
	}
	if spec != nil && triggerKind(spec.Trigger) != jobsv0.TriggerKindManual {
		return nil, nil, false, errdefs.InvalidParameter(errors.New("create-and-run serves manual jobs only; register schedule jobs with create"))
	}

	var job *jobsv0.Job
	switch {
	case name == "":
		// No name: always a fresh job under a generated name.
		var err error
		if job, err = m.createNamed(ctx, spec); err != nil {
			return nil, nil, false, err
		}
		created = true
	case spec == nil:
		// The request field is a name, strictly: the create branches treat
		// it as one, so resolving IDs here would be an observable asymmetry.
		existing, err := m.store.JobByName(name)
		if err != nil {
			return nil, nil, false, err
		}
		if triggerKind(existing.Spec.Trigger) != jobsv0.TriggerKindManual {
			return nil, nil, false, errdefs.InvalidParameter(fmt.Errorf("job %s is not a manual job", name))
		}
		job = existing
	default:
		var err error
		if job, created, err = m.Create(ctx, name, spec); err != nil {
			return nil, nil, false, err
		}
	}

	run, err := m.fireManual(ctx, job.ID)
	if err != nil {
		return nil, nil, false, err
	}
	job, err = m.Inspect(ctx, job.ID)
	if err != nil {
		return nil, nil, false, err
	}
	return job, run, created, nil
}

// createNamed registers spec under a generated adjective_noun name,
// retrying on collisions like the daemon does for container names. A random
// name landing on an existing job with an identical spec also counts as a
// collision: adopting that job would silently hand out foreign history.
func (m *Manager) createNamed(ctx context.Context, spec *jobsv0.JobSpec) (*jobsv0.Job, error) {
	var lastErr error
	for i := range nameGenerationRetries {
		name := namesgenerator.GetRandomName(i)
		job, created, err := m.Create(ctx, name, spec)
		switch {
		case err == nil && created:
			return job, nil
		case err == nil:
			lastErr = fmt.Errorf("generated name %s is already in use", name)
		case cerrdefs.IsAlreadyExists(err) || cerrdefs.IsConflict(err):
			lastErr = err
		default:
			return nil, err
		}
	}
	return nil, fmt.Errorf("generating a unique job name: %w", lastErr)
}

// resolveJob resolves a job reference, trying IDs first and names second,
// like the container API does.
func (m *Manager) resolveJob(ref string) (*jobsv0.Job, error) {
	if ref == "" {
		return nil, errdefs.InvalidParameter(errors.New("job reference must not be empty"))
	}
	job, err := m.store.Job(ref)
	if err == nil {
		return job, nil
	}
	return m.store.JobByName(ref)
}

// composeJob attaches the derived LatestRun to a stored job.
func (m *Manager) composeJob(job *jobsv0.Job) *jobsv0.Job {
	if latest, err := m.store.LatestRun(job.ID); err == nil {
		job.LatestRun = latest
	}
	return job
}

// validateSpec checks everything the contract rejects at registration time
// and returns the decoded container definition.
func (m *Manager) validateSpec(spec *jobsv0.JobSpec) (*container.CreateRequest, error) {
	if spec == nil {
		return nil, errdefs.InvalidParameter(errors.New("a job spec is required"))
	}
	if err := validateTrigger(spec.Trigger, m.validateCron); err != nil {
		return nil, errdefs.InvalidParameter(err)
	}
	if spec.TimeoutSeconds < 0 {
		return nil, errdefs.InvalidParameter(errors.New("timeout must not be negative"))
	}
	for label := range spec.Labels {
		if strings.HasPrefix(label, "com.docker.job.") {
			return nil, errdefs.InvalidParameter(fmt.Errorf("label %s is reserved", label))
		}
	}

	decoded, err := decodeContainerSpec(spec.ContainerSpec)
	if err != nil {
		return nil, errdefs.InvalidParameter(fmt.Errorf("container spec: %w", err))
	}
	if decoded.Image == "" {
		return nil, errdefs.InvalidParameter(errors.New("container spec: an image is required"))
	}
	for label := range decoded.Labels {
		if strings.HasPrefix(label, "com.docker.job.") {
			return nil, errdefs.InvalidParameter(fmt.Errorf("container spec: label %s is reserved", label))
		}
	}
	if hc := decoded.HostConfig; hc != nil {
		// AutoRemove races the exit-code capture the run record depends on;
		// RemoveOnSuccess/RemoveOnFailure are the job-level equivalents.
		if hc.AutoRemove {
			return nil, errdefs.InvalidParameter(errors.New("container spec: AutoRemove is not supported for jobs; use RemoveOnSuccess or RemoveOnFailure"))
		}
		// A job is expected to terminate; keep-alive restart policies would
		// make the run never reach an outcome. The same goes for on-failure
		// without a retry cap, which Docker treats as retry-forever.
		switch hc.RestartPolicy.Name {
		case "", container.RestartPolicyDisabled:
		case container.RestartPolicyOnFailure:
			if hc.RestartPolicy.MaximumRetryCount <= 0 {
				return nil, errdefs.InvalidParameter(errors.New("container spec: on-failure requires a maximum retry count for jobs; unbounded retries would keep the run from ever completing"))
			}
		default:
			return nil, errdefs.InvalidParameter(fmt.Errorf("container spec: restart policy %s is not supported for jobs; use no or on-failure", hc.RestartPolicy.Name))
		}
	}
	return decoded, nil
}

// validateTrigger enforces the trigger sum-type rules of the API contract.
func validateTrigger(trigger *jobsv0.Trigger, validateCron func(string) error) error {
	if trigger == nil {
		return nil // the manual shorthand
	}
	switch {
	case trigger.Manual && trigger.Schedule != nil:
		return errors.New("trigger declares more than one kind")
	case trigger.Schedule != nil:
		sched := trigger.Schedule
		if err := validateCron(sched.Cron); err != nil {
			return fmt.Errorf("schedule: %w", err)
		}
		if sched.Timezone != "" {
			if _, err := time.LoadLocation(sched.Timezone); err != nil {
				return fmt.Errorf("schedule: unknown timezone %s", sched.Timezone)
			}
		}
		switch sched.Concurrency {
		case "", jobsv0.ConcurrencyForbid, jobsv0.ConcurrencyQueue:
		default:
			return fmt.Errorf("schedule: unsupported concurrency policy %s", sched.Concurrency)
		}
		switch sched.MissedFires {
		case "", jobsv0.MissedFiresOne, jobsv0.MissedFiresSkip:
		default:
			return fmt.Errorf("schedule: unsupported missed-fires policy %s", sched.MissedFires)
		}
		return nil
	case trigger.Manual:
		return nil
	default:
		// Deliberate forward-compatibility failure: a trigger kind from a
		// newer contract version reaches an older daemon as an empty
		// message, because protobuf drops unknown fields. Registering it
		// as anything would silently disarm it.
		return errors.New("trigger declares no known kind; the daemon may be too old for this trigger type")
	}
}

// validateJobName applies the container-name character rules to job names.
func validateJobName(name string) error {
	if name == "" {
		return errors.New("a job name is required")
	}
	if !names.RestrictedNamePattern.MatchString(name) {
		return fmt.Errorf("invalid job name %q, only %s are allowed", name, names.RestrictedNameChars)
	}
	return nil
}

// decodeContainerSpec decodes the JSON container definition, in the exact
// format of the container-create API request body. Unknown fields are
// rejected so a typo fails registration instead of silently altering the
// container.
func decodeContainerSpec(raw []byte) (*container.CreateRequest, error) {
	if len(raw) == 0 {
		return nil, errors.New("a container definition is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	req := &container.CreateRequest{}
	if err := decoder.Decode(req); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("unexpected trailing data after the container definition")
	}
	if req.Config == nil {
		req.Config = &container.Config{}
	}
	return req, nil
}

// canonicalSpec is the hashing form of a job spec: daemon defaults applied,
// container definition re-encoded from its decoded form so JSON formatting
// differences (whitespace, key order — encoding/json sorts map keys) do not
// change a job's identity. Structural presence still matters: an explicit
// empty HostConfig decodes to a non-nil pointer and hashes differently from
// an absent one, a conservative trade-off over normalizing every substruct.
type canonicalSpec struct {
	ContainerSpec   *container.CreateRequest
	TriggerKind     string
	Schedule        *jobsv0.ScheduleTrigger `json:",omitempty"`
	Labels          map[string]string       `json:",omitempty"`
	TimeoutSeconds  int64
	RemoveOnSuccess bool
	RemoveOnFailure bool
	RunHistoryLimit uint32
}

// specHash computes the canonical identity hash of a validated spec.
func specHash(spec *jobsv0.JobSpec, decoded *container.CreateRequest) (string, error) {
	canonical := canonicalSpec{
		ContainerSpec:   decoded,
		TriggerKind:     triggerKind(spec.Trigger),
		Labels:          spec.Labels,
		TimeoutSeconds:  spec.TimeoutSeconds,
		RemoveOnSuccess: spec.RemoveOnSuccess,
		RemoveOnFailure: spec.RemoveOnFailure,
		RunHistoryLimit: spec.RunHistoryLimit,
	}
	if canonical.RunHistoryLimit == 0 {
		canonical.RunHistoryLimit = DefaultRunHistoryLimit
	}
	if spec.Trigger != nil && spec.Trigger.Schedule != nil {
		sched := *spec.Trigger.Schedule
		if sched.Timezone == "" {
			sched.Timezone = "UTC"
		}
		if sched.Concurrency == "" {
			sched.Concurrency = jobsv0.ConcurrencyForbid
		}
		if sched.MissedFires == "" {
			sched.MissedFires = jobsv0.MissedFiresOne
		}
		canonical.Schedule = &sched
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encoding canonical spec: %w", err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data)), nil
}

// triggerKind names a validated trigger's kind; a nil trigger is the manual
// shorthand.
func triggerKind(trigger *jobsv0.Trigger) string {
	if trigger != nil && trigger.Schedule != nil {
		return jobsv0.TriggerKindSchedule
	}
	return jobsv0.TriggerKindManual
}
