package jobs

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// newTestService serves the jobs extension the way the daemon does — through
// serviceExposer on a gRPC server — and returns a generated client dialing
// it, so the tests cover the exact path an API consumer takes.
func newTestService(t *testing.T) (jobspb.JobsClient, *Extension, *fakeBackend) {
	t.Helper()
	ext := NewExtension(filepath.Join(t.TempDir(), "jobs"))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()
		assert.Check(t, ext.shutdownManager(ctx))
	})

	// Not t.TempDir(): it embeds the test name, and a long one pushes the
	// socket path past the 104-byte sun_path limit on macOS.
	sockDir, err := os.MkdirTemp("", "jobsd")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	lis, err := net.Listen("unix", filepath.Join(sockDir, "d.sock"))
	assert.NilError(t, err)
	gs := grpc.NewServer()
	serviceExposer{ext: ext}.RegisterServices(gs)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("unix://"+filepath.Join(sockDir, "d.sock"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { conn.Close() })
	return jobspb.NewJobsClient(conn), ext, newFakeBackend()
}

func TestServiceUnavailableBeforeActivation(t *testing.T) {
	client, _, _ := newTestService(t)
	_, err := client.List(t.Context(), &jobspb.ListRequest{})
	assert.Check(t, is.Equal(status.Code(err), codes.Unavailable))
}

func TestServiceJourney(t *testing.T) {
	client, ext, fake := newTestService(t)
	assert.NilError(t, ext.Activate(t.Context(), fake))

	// Register a manual job; re-applying is a no-op, a different spec under
	// the same name is a conflict surfaced as AlreadyExists.
	spec := &jobspb.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`)}
	created, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "backup", Spec: spec})
	assert.NilError(t, err)
	assert.Check(t, created.GetCreated())
	again, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "backup", Spec: spec})
	assert.NilError(t, err)
	assert.Check(t, !again.GetCreated())
	_, err = client.Create(t.Context(), &jobspb.CreateRequest{Name: "backup", Spec: &jobspb.JobSpec{ContainerSpec: []byte(`{"Image":"alpine"}`)}})
	assert.Check(t, is.Equal(status.Code(err), codes.AlreadyExists))

	// Validation failures surface as InvalidArgument, unknown jobs as
	// NotFound.
	_, err = client.Create(t.Context(), &jobspb.CreateRequest{Name: "bad", Spec: &jobspb.JobSpec{ContainerSpec: []byte(`{}`)}})
	assert.Check(t, is.Equal(status.Code(err), codes.InvalidArgument))
	_, err = client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "ghost"})
	assert.Check(t, is.Equal(status.Code(err), codes.NotFound))

	// Execute; a second run while the first is in flight is refused with
	// FailedPrecondition.
	runReply, err := client.Run(t.Context(), &jobspb.RunRequest{JobRef: "backup"})
	assert.NilError(t, err)
	run := runReply.GetRun()
	assert.Check(t, is.Equal(run.GetState(), "running"))
	_, err = client.Run(t.Context(), &jobspb.RunRequest{JobRef: "backup"})
	assert.Check(t, is.Equal(status.Code(err), codes.FailedPrecondition))

	// Complete the run and observe it through Wait.
	fake.exit(run.GetContainerId(), 0, nil)
	waited, err := client.Wait(t.Context(), &jobspb.WaitRequest{JobRef: "backup", RunRef: run.GetId()})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(waited.GetRun().GetState(), "succeeded"))
	assert.Assert(t, waited.GetRun().GetExitCode() != nil)
	assert.Check(t, is.Equal(waited.GetRun().GetExitCode().GetValue(), int64(0)))

	// List, run history, and removal round out the journey.
	list, err := client.List(t.Context(), &jobspb.ListRequest{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(list.GetJobs()), 1))
	runs, err := client.ListRuns(t.Context(), &jobspb.ListRunsRequest{JobRef: "backup"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(runs.GetRuns()), 1))
	_, err = client.Remove(t.Context(), &jobspb.RemoveRequest{JobRef: "backup"})
	assert.NilError(t, err)
	_, err = client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "backup"})
	assert.Check(t, is.Equal(status.Code(err), codes.NotFound))
}

func TestExtensionRejectsDoubleActivation(t *testing.T) {
	_, ext, fake := newTestService(t)
	assert.NilError(t, ext.Activate(t.Context(), fake))
	// A second activation would leak the first manager's scheduler and
	// have two stores writing the same directory.
	assert.Check(t, is.ErrorContains(ext.Activate(t.Context(), fake), "already active"))
}

func TestServicePersistsAcrossActivations(t *testing.T) {
	client, ext, fake := newTestService(t)
	assert.NilError(t, ext.Activate(t.Context(), fake))

	_, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "durable", Spec: &jobspb.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`)}})
	assert.NilError(t, err)

	// A shutdown/activate cycle — the extension's restart — reloads the
	// job from its on-disk record.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	assert.NilError(t, ext.shutdownManager(ctx))
	_, err = client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "durable"})
	assert.Check(t, is.Equal(status.Code(err), codes.Unavailable))

	assert.NilError(t, ext.Activate(t.Context(), fake))
	inspected, err := client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "durable"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.GetJob().GetName(), "durable"))
}
