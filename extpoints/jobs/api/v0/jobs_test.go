package jobsv0_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// The tests below round-trip every message field through the generated wire
// layer: contract structs on the server side, protoc-style messages on the
// client side. They catch pb-tag mistakes and generated-converter drift that
// compilation alone cannot: a field silently dropped by the converters still
// compiles.

// fixtureSpec populates every JobSpec field with a distinct value.
var fixtureSpec = &jobsv0.JobSpec{
	ContainerSpec: []byte(`{"Image":"busybox"}`),
	Trigger: &jobsv0.Trigger{
		Schedule: &jobsv0.ScheduleTrigger{
			Cron:        "0 3 * * *",
			Timezone:    "Europe/Paris",
			Concurrency: jobsv0.ConcurrencyQueue,
			MissedFires: jobsv0.MissedFiresSkip,
		},
	},
	Labels:          map[string]string{"com.example.one": "1", "com.example.two": "2"},
	TimeoutSeconds:  3600,
	RemoveOnSuccess: true,
	RemoveOnFailure: true,
	RunHistoryLimit: 42,
}

// fixtureRun populates every Run field. ExitCode deliberately wraps zero so
// the tests prove that a zero exit code is distinguishable from no exit code.
var fixtureRun = &jobsv0.Run{
	ID:             "r1",
	JobID:          "j1",
	Iteration:      7,
	ContainerID:    "c1",
	ContainerGone:  true,
	State:          jobsv0.RunStateSucceeded,
	CreatedAtNano:  1,
	StartedAtNano:  2,
	FinishedAtNano: 3,
	ExitCode:       &jobsv0.ExitCode{Value: 0},
	Error:          "context for a lost container",
	Trigger: &jobsv0.TriggerEvidence{
		Kind:            jobsv0.TriggerKindSchedule,
		ScheduledAtNano: 4,
		FiredAtNano:     5,
	},
}

// fixtureJob populates every Job field, embedding the spec and run fixtures.
var fixtureJob = &jobsv0.Job{
	ID:             "j1",
	Name:           "nightly-backup",
	Spec:           fixtureSpec,
	SpecHash:       "sha256:0123",
	State:          jobsv0.JobStateRunning,
	Paused:         true,
	NextFireAtNano: 8,
	CreatedAtNano:  9,
	UpdatedAtNano:  10,
	LatestRun:      fixtureRun,
}

// echoJobs implements the contract by deriving every reply from its request
// and the shared fixtures, so the client side can assert exact values.
type echoJobs struct{}

func (echoJobs) Create(_ context.Context, req *jobsv0.CreateRequest) (*jobsv0.CreateReply, error) {
	job := *fixtureJob
	job.Name = req.Name
	job.Spec = req.Spec
	return &jobsv0.CreateReply{Job: &job, Created: true}, nil
}

func (echoJobs) Run(_ context.Context, req *jobsv0.RunRequest) (*jobsv0.RunReply, error) {
	run := *fixtureRun
	run.JobID = req.JobRef
	run.ContainerGone = req.Reschedule
	return &jobsv0.RunReply{Run: &run}, nil
}

func (echoJobs) CreateAndRun(_ context.Context, req *jobsv0.CreateAndRunRequest) (*jobsv0.CreateAndRunReply, error) {
	job := *fixtureJob
	job.Name = req.Name
	job.Spec = req.Spec
	return &jobsv0.CreateAndRunReply{Job: &job, Run: fixtureRun, Created: false}, nil
}

func (echoJobs) Inspect(_ context.Context, req *jobsv0.InspectRequest) (*jobsv0.InspectReply, error) {
	if req.JobRef != fixtureJob.Name {
		return nil, status.Errorf(codes.NotFound, "no such job: %s", req.JobRef)
	}
	return &jobsv0.InspectReply{Job: fixtureJob}, nil
}

func (echoJobs) List(_ context.Context, req *jobsv0.ListRequest) (*jobsv0.ListReply, error) {
	// Echo the filters through job names so the test proves every filter
	// field crossed the wire.
	var jobs []jobsv0.Job
	for _, name := range req.Names {
		jobs = append(jobs, jobsv0.Job{Name: name})
	}
	for _, l := range req.Labels {
		jobs = append(jobs, jobsv0.Job{Name: l})
	}
	for _, s := range req.States {
		jobs = append(jobs, jobsv0.Job{Name: s})
	}
	for _, k := range req.TriggerKinds {
		jobs = append(jobs, jobsv0.Job{Name: k})
	}
	if req.Paused != "" {
		jobs = append(jobs, jobsv0.Job{Name: req.Paused})
	}
	for _, s := range req.LatestRunStates {
		jobs = append(jobs, jobsv0.Job{Name: s})
	}
	return &jobsv0.ListReply{Jobs: jobs}, nil
}

func (echoJobs) Pause(_ context.Context, req *jobsv0.PauseRequest) error {
	if req.JobRef == "" {
		return status.Error(codes.InvalidArgument, "empty job reference")
	}
	return nil
}

func (echoJobs) Resume(_ context.Context, _ *jobsv0.ResumeRequest) error {
	return nil
}

func (echoJobs) Cancel(_ context.Context, req *jobsv0.CancelRequest) (*jobsv0.CancelReply, error) {
	return &jobsv0.CancelReply{RunID: "cancelled-in-" + req.JobRef}, nil
}

func (echoJobs) Remove(_ context.Context, req *jobsv0.RemoveRequest) error {
	if req.RunsRemoval == "bogus" {
		return status.Error(codes.InvalidArgument, "invalid runs-removal mode")
	}
	return nil
}

func (echoJobs) Prune(_ context.Context, req *jobsv0.PruneRequest) (*jobsv0.PruneReply, error) {
	return &jobsv0.PruneReply{RemovedJobIDs: req.Labels}, nil
}

func (echoJobs) ListRuns(_ context.Context, req *jobsv0.ListRunsRequest) (*jobsv0.ListRunsReply, error) {
	runs := make([]jobsv0.Run, req.Limit)
	for i := range runs {
		runs[i] = *fixtureRun
		runs[i].Iteration = uint64(i + 1)
	}
	return &jobsv0.ListRunsReply{Runs: runs, NextCursor: req.Before, CursorStale: true}, nil
}

func (echoJobs) InspectRun(_ context.Context, req *jobsv0.InspectRunRequest) (*jobsv0.InspectRunReply, error) {
	run := *fixtureRun
	run.JobID = req.JobRef
	run.ID = req.RunRef
	return &jobsv0.InspectRunReply{Run: &run}, nil
}

func (echoJobs) Wait(_ context.Context, req *jobsv0.WaitRequest) (*jobsv0.WaitReply, error) {
	run := *fixtureRun
	run.JobID = req.JobRef
	run.ID = req.RunRef
	run.State = req.Condition
	return &jobsv0.WaitReply{Run: &run}, nil
}

// newClient serves echoJobs through the generated server adapter on a local
// socket and returns the generated client, exercising the same path as an
// extension exposed on the daemon socket.
func newClient(t *testing.T) jobspb.JobsClient {
	t.Helper()
	// Unix socket paths are capped at ~104 bytes on macOS and t.TempDir()
	// embeds the test name; keep test function names short.
	sock := filepath.Join(t.TempDir(), "jobs.sock")
	lis, err := net.Listen("unix", sock)
	assert.NilError(t, err)

	gs := grpc.NewServer()
	jobspb.ServerPoint.Register(gs, echoJobs{})
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { conn.Close() })
	return jobspb.NewJobsClient(conn)
}

// checkSpec asserts every JobSpec field survived the trip.
func checkSpec(t *testing.T, got *jobspb.JobSpec, want *jobsv0.JobSpec) {
	t.Helper()
	assert.Assert(t, got != nil)
	assert.Check(t, is.DeepEqual(got.GetContainerSpec(), want.ContainerSpec))
	assert.Assert(t, got.GetTrigger() != nil)
	assert.Check(t, is.Equal(got.GetTrigger().GetManual(), want.Trigger.Manual))
	sched := got.GetTrigger().GetSchedule()
	assert.Assert(t, sched != nil)
	assert.Check(t, is.Equal(sched.GetCron(), want.Trigger.Schedule.Cron))
	assert.Check(t, is.Equal(sched.GetTimezone(), want.Trigger.Schedule.Timezone))
	assert.Check(t, is.Equal(sched.GetConcurrency(), want.Trigger.Schedule.Concurrency))
	assert.Check(t, is.Equal(sched.GetMissedFires(), want.Trigger.Schedule.MissedFires))
	assert.Check(t, is.DeepEqual(got.GetLabels(), want.Labels))
	assert.Check(t, is.Equal(got.GetTimeoutSeconds(), want.TimeoutSeconds))
	assert.Check(t, is.Equal(got.GetRemoveOnSuccess(), want.RemoveOnSuccess))
	assert.Check(t, is.Equal(got.GetRemoveOnFailure(), want.RemoveOnFailure))
	assert.Check(t, is.Equal(got.GetRunHistoryLimit(), want.RunHistoryLimit))
}

// checkRun asserts every Run field survived the trip.
func checkRun(t *testing.T, got *jobspb.Run, want *jobsv0.Run) {
	t.Helper()
	assert.Assert(t, got != nil)
	assert.Check(t, is.Equal(got.GetId(), want.ID))
	assert.Check(t, is.Equal(got.GetJobId(), want.JobID))
	assert.Check(t, is.Equal(got.GetIteration(), want.Iteration))
	assert.Check(t, is.Equal(got.GetContainerId(), want.ContainerID))
	assert.Check(t, is.Equal(got.GetContainerGone(), want.ContainerGone))
	assert.Check(t, is.Equal(got.GetState(), want.State))
	assert.Check(t, is.Equal(got.GetCreatedAtNano(), want.CreatedAtNano))
	assert.Check(t, is.Equal(got.GetStartedAtNano(), want.StartedAtNano))
	assert.Check(t, is.Equal(got.GetFinishedAtNano(), want.FinishedAtNano))
	// A present-but-zero exit code must arrive present, not absent.
	assert.Assert(t, got.GetExitCode() != nil)
	assert.Check(t, is.Equal(got.GetExitCode().GetValue(), want.ExitCode.Value))
	assert.Check(t, is.Equal(got.GetError(), want.Error))
	assert.Assert(t, got.GetTrigger() != nil)
	assert.Check(t, is.Equal(got.GetTrigger().GetKind(), want.Trigger.Kind))
	assert.Check(t, is.Equal(got.GetTrigger().GetScheduledAtNano(), want.Trigger.ScheduledAtNano))
	assert.Check(t, is.Equal(got.GetTrigger().GetFiredAtNano(), want.Trigger.FiredAtNano))
}

// checkJob asserts every Job field survived the trip.
func checkJob(t *testing.T, got *jobspb.Job, want *jobsv0.Job) {
	t.Helper()
	assert.Assert(t, got != nil)
	assert.Check(t, is.Equal(got.GetId(), want.ID))
	assert.Check(t, is.Equal(got.GetName(), want.Name))
	checkSpec(t, got.GetSpec(), want.Spec)
	assert.Check(t, is.Equal(got.GetSpecHash(), want.SpecHash))
	assert.Check(t, is.Equal(got.GetState(), want.State))
	assert.Check(t, is.Equal(got.GetPaused(), want.Paused))
	assert.Check(t, is.Equal(got.GetNextFireAtNano(), want.NextFireAtNano))
	assert.Check(t, is.Equal(got.GetCreatedAtNano(), want.CreatedAtNano))
	assert.Check(t, is.Equal(got.GetUpdatedAtNano(), want.UpdatedAtNano))
	checkRun(t, got.GetLatestRun(), want.LatestRun)
}

// specToProto builds the protogen JobSpec matching fixtureSpec for requests.
func specToProto() *jobspb.JobSpec {
	return &jobspb.JobSpec{
		ContainerSpec: fixtureSpec.ContainerSpec,
		Trigger: &jobspb.Trigger{Schedule: &jobspb.ScheduleTrigger{
			Cron:        fixtureSpec.Trigger.Schedule.Cron,
			Timezone:    fixtureSpec.Trigger.Schedule.Timezone,
			Concurrency: fixtureSpec.Trigger.Schedule.Concurrency,
			MissedFires: fixtureSpec.Trigger.Schedule.MissedFires,
		}},
		Labels:          fixtureSpec.Labels,
		TimeoutSeconds:  fixtureSpec.TimeoutSeconds,
		RemoveOnSuccess: fixtureSpec.RemoveOnSuccess,
		RemoveOnFailure: fixtureSpec.RemoveOnFailure,
		RunHistoryLimit: fixtureSpec.RunHistoryLimit,
	}
}

func TestRoundTripJobGraph(t *testing.T) {
	client := newClient(t)

	reply, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: fixtureJob.Name, Spec: specToProto()})
	assert.NilError(t, err)
	assert.Check(t, reply.GetCreated())
	checkJob(t, reply.GetJob(), fixtureJob)
}

func TestRoundTripManualTrigger(t *testing.T) {
	client := newClient(t)

	// A manual trigger must arrive as an explicit Manual kind, not as an
	// empty Trigger: an empty Trigger is the reject-unknown-kinds signal.
	spec := specToProto()
	spec.Trigger = &jobspb.Trigger{Manual: true}
	reply, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "manual-job", Spec: spec})
	assert.NilError(t, err)
	trigger := reply.GetJob().GetSpec().GetTrigger()
	assert.Assert(t, trigger != nil)
	assert.Check(t, trigger.GetManual())
	assert.Check(t, is.Nil(trigger.GetSchedule()))
}

func TestRoundTripRunLifecycle(t *testing.T) {
	client := newClient(t)

	runReply, err := client.Run(t.Context(), &jobspb.RunRequest{JobRef: "job-a", Reschedule: true})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(runReply.GetRun().GetJobId(), "job-a"))
	assert.Check(t, runReply.GetRun().GetContainerGone())

	waitReply, err := client.Wait(t.Context(), &jobspb.WaitRequest{JobRef: "job-a", RunRef: "run-b", Condition: jobsv0.WaitConditionRunning})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(waitReply.GetRun().GetJobId(), "job-a"))
	assert.Check(t, is.Equal(waitReply.GetRun().GetId(), "run-b"))
	assert.Check(t, is.Equal(waitReply.GetRun().GetState(), jobsv0.WaitConditionRunning))

	inspectReply, err := client.InspectRun(t.Context(), &jobspb.InspectRunRequest{JobRef: "j1", RunRef: "r1"})
	assert.NilError(t, err)
	checkRun(t, inspectReply.GetRun(), fixtureRun)

	cancelReply, err := client.Cancel(t.Context(), &jobspb.CancelRequest{JobRef: "job-a"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cancelReply.GetRunId(), "cancelled-in-job-a"))

	carReply, err := client.CreateAndRun(t.Context(), &jobspb.CreateAndRunRequest{Name: fixtureJob.Name, Spec: specToProto()})
	assert.NilError(t, err)
	assert.Check(t, !carReply.GetCreated())
	checkJob(t, carReply.GetJob(), fixtureJob)
	checkRun(t, carReply.GetRun(), fixtureRun)
}

func TestRoundTripListFilters(t *testing.T) {
	client := newClient(t)

	reply, err := client.List(t.Context(), &jobspb.ListRequest{
		Names:           []string{"n1"},
		Labels:          []string{"l1=v1"},
		States:          []string{jobsv0.JobStateIdle},
		TriggerKinds:    []string{jobsv0.TriggerKindSchedule},
		Paused:          "true",
		LatestRunStates: []string{jobsv0.RunStateFailed},
	})
	assert.NilError(t, err)
	var names []string
	for _, j := range reply.GetJobs() {
		names = append(names, j.GetName())
	}
	assert.Check(t, is.DeepEqual(names, []string{"n1", "l1=v1", jobsv0.JobStateIdle, jobsv0.TriggerKindSchedule, "true", jobsv0.RunStateFailed}))
}

func TestRoundTripRunsPagination(t *testing.T) {
	client := newClient(t)

	reply, err := client.ListRuns(t.Context(), &jobspb.ListRunsRequest{JobRef: "j1", Limit: 3, Before: "cursor-1"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(reply.GetRuns()), 3))
	for i, run := range reply.GetRuns() {
		assert.Check(t, is.Equal(run.GetIteration(), uint64(i+1)))
	}
	assert.Check(t, is.Equal(reply.GetNextCursor(), "cursor-1"))
	assert.Check(t, reply.GetCursorStale())
}

func TestRoundTripBareErrorMethods(t *testing.T) {
	client := newClient(t)

	_, err := client.Pause(t.Context(), &jobspb.PauseRequest{JobRef: "j1"})
	assert.NilError(t, err)
	_, err = client.Resume(t.Context(), &jobspb.ResumeRequest{JobRef: "j1"})
	assert.NilError(t, err)
	_, err = client.Remove(t.Context(), &jobspb.RemoveRequest{JobRef: "j1", RunsRemoval: jobsv0.RunsRemoveFinished})
	assert.NilError(t, err)

	pruneReply, err := client.Prune(t.Context(), &jobspb.PruneRequest{Labels: []string{"a=b", "c"}})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(pruneReply.GetRemovedJobIds(), []string{"a=b", "c"}))
}

func TestStatusCodesCrossTheWire(t *testing.T) {
	client := newClient(t)

	_, err := client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "does-not-exist"})
	assert.Check(t, is.Equal(status.Code(err), codes.NotFound))

	// Errors from bare-error methods must carry their status too.
	_, err = client.Pause(t.Context(), &jobspb.PauseRequest{})
	assert.Check(t, is.Equal(status.Code(err), codes.InvalidArgument))

	_, err = client.Remove(t.Context(), &jobspb.RemoveRequest{JobRef: "j1", RunsRemoval: "bogus"})
	assert.Check(t, is.Equal(status.Code(err), codes.InvalidArgument))
}
