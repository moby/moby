package jobs

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/extensions"
	"github.com/moby/extensions/host"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	runtimev0 "github.com/moby/moby/v2/extpoints/runtime/v0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

// newTestService serves the jobs extension the way the daemon does — the
// generated ServerPoint wiring registered on a gRPC server — and returns a
// generated client dialing it, so the tests cover the exact path an API
// consumer takes.
func newTestService(t *testing.T) (jobspb.JobsClient, *Extension, *fakeBackend) {
	t.Helper()
	ext := NewExtension(filepath.Join(t.TempDir(), "jobs"))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()
		assert.Check(t, ext.shutdownManager(ctx))
	})

	return serveJobsClient(t, func(r grpc.ServiceRegistrar) {
		jobspb.ServerPoint.Register(r, &service{ext: ext})
	}), ext, newFakeBackend()
}

// serveJobsClient serves whatever register installs on a gRPC server over a
// unix socket and returns a generated client dialing it.
func serveJobsClient(t *testing.T, register func(grpc.ServiceRegistrar)) jobspb.JobsClient {
	t.Helper()
	// Not t.TempDir(): it embeds the test name, and a long one pushes the
	// socket path past the 104-byte sun_path limit on macOS.
	sockDir, err := os.MkdirTemp("", "jobsd")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	lis, err := net.Listen("unix", filepath.Join(sockDir, "d.sock"))
	assert.NilError(t, err)
	gs := grpc.NewServer()
	register(gs)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("unix://"+filepath.Join(sockDir, "d.sock"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { conn.Close() })
	return jobspb.NewJobsClient(conn)
}

// activateExtension activates ext on backend the way init's background
// goroutine does once the runtime reports ready.
func activateExtension(ctx context.Context, ext *Extension, backend Backend) error {
	manager, err := ext.buildManager(ctx, backend)
	if err != nil {
		return err
	}
	return ext.activate(ctx, manager)
}

func TestServiceUnavailableBeforeActivation(t *testing.T) {
	client, _, _ := newTestService(t)
	_, err := client.List(t.Context(), &jobspb.ListRequest{})
	assert.Check(t, is.Equal(status.Code(err), codes.Unavailable))
}

func TestServiceJourney(t *testing.T) {
	client, ext, fake := newTestService(t)
	assert.NilError(t, activateExtension(t.Context(), ext, fake))

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
	assert.NilError(t, activateExtension(t.Context(), ext, fake))
	// A second activation would leak the first manager's scheduler and
	// have two stores writing the same directory.
	assert.Check(t, is.ErrorContains(activateExtension(t.Context(), ext, fake), "already active"))
}

func TestServicePersistsAcrossActivations(t *testing.T) {
	client, ext, fake := newTestService(t)
	assert.NilError(t, activateExtension(t.Context(), ext, fake))

	_, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "durable", Spec: &jobspb.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`)}})
	assert.NilError(t, err)

	// A shutdown/activate cycle — the extension's restart — reloads the
	// job from its on-disk record.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	assert.NilError(t, ext.shutdownManager(ctx))
	_, err = client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "durable"})
	assert.Check(t, is.Equal(status.Code(err), codes.Unavailable))

	assert.NilError(t, activateExtension(t.Context(), ext, fake))
	inspected, err := client.Inspect(t.Context(), &jobspb.InspectRequest{JobRef: "durable"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspected.GetJob().GetName(), "durable"))
}

// TestExtensionThroughHost drives the extension through a real extension
// host, wired the way the daemon does it: the runtime point provided by a
// builtin extension backed by a fake, the Jobs point offered through
// service.v0 and adapted to gRPC by the host with the generated wiring. It
// catches drift between the extension's declaration and the framework's
// publication contract that direct calls into the extension cannot see.
func TestExtensionThroughHost(t *testing.T) {
	fake := newFakeBackend()
	runtimeExt := extensions.New(extensions.Declaration{
		ID:        "org.mobyproject.test-runtime.v1",
		Providers: []extensions.Provider{runtimev0.Point.Provide(Backend(fake))},
	})
	ext := NewExtension(filepath.Join(t.TempDir(), "jobs"))
	h, err := host.New(t.Context(),
		host.WithRuntimeDir(t.TempDir()),
		host.WithExtensions(runtimeExt, ext),
		host.WithPointServers(jobspb.ServerPoint),
		host.WithProviderPolicy(host.PointPolicyFunc(func(extensions.ExtensionIdentity, extensions.PointID) host.PointPolicyResult {
			return host.Allow()
		})),
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()
		assert.Check(t, h.Shutdown(ctx))
	})
	client := serveJobsClient(t, h.RegisterInProcessServices)

	// Activation is asynchronous behind the runtime's readiness gate; the
	// API answers Unavailable until it completes.
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		_, err := client.List(t.Context(), &jobspb.ListRequest{})
		switch status.Code(err) {
		case codes.OK:
			return poll.Success()
		case codes.Unavailable:
			return poll.Continue("jobs API not activated yet")
		default:
			return poll.Error(err)
		}
	})

	created, err := client.Create(t.Context(), &jobspb.CreateRequest{Name: "hosted", Spec: &jobspb.JobSpec{ContainerSpec: []byte(`{"Image":"busybox"}`)}})
	assert.NilError(t, err)
	assert.Check(t, created.GetCreated())
}

// staticResolver resolves every point to a single fixed provider,
// impersonating the extension host in tests of the init path.
type staticResolver struct {
	impl any
}

func (r staticResolver) Provider(extensions.PointID, extensions.ExtensionID) (any, error) {
	return r.impl, nil
}

func (r staticResolver) Providers(extensions.PointID) []extensions.ResolvedProvider {
	return []extensions.ResolvedProvider{{Impl: r.impl}}
}

// parkedBackend never reports ready; it records how its Ready wait ended.
type parkedBackend struct {
	*fakeBackend
	released chan error
}

func (b *parkedBackend) Ready(ctx context.Context) error {
	<-ctx.Done()
	b.released <- ctx.Err()
	return ctx.Err()
}

// TestExtensionShutdownReleasesPendingActivation shuts the extension down
// while its activation is still parked on the runtime's readiness gate: the
// goroutine must be released and no manager may start afterwards.
func TestExtensionShutdownReleasesPendingActivation(t *testing.T) {
	ext := NewExtension(filepath.Join(t.TempDir(), "jobs"))
	backend := &parkedBackend{fakeBackend: newFakeBackend(), released: make(chan error, 1)}
	assert.NilError(t, ext.init(t.Context(), nil, staticResolver{impl: Backend(backend)}))

	assert.NilError(t, ext.shutdownManager(t.Context()))
	select {
	case err := <-backend.released:
		assert.Check(t, is.ErrorIs(err, context.Canceled))
	case <-time.After(10 * time.Second):
		t.Fatal("activation goroutine still parked in Ready after shutdown")
	}
	ext.mu.Lock()
	manager := ext.manager
	ext.mu.Unlock()
	assert.Check(t, is.Nil(manager))
}
