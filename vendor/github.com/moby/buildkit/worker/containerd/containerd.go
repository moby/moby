package containerd

import (
	"context"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/gc"
	"github.com/containerd/containerd/leases"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/platforms"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/oci"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/util/winlayers"
	"github.com/moby/buildkit/worker/base"
	wlabel "github.com/moby/buildkit/worker/label"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

type RuntimeInfo = containerdexecutor.RuntimeInfo

type WorkerOptions struct {
	Root            string
	Address         string
	SnapshotterName string
	Namespace       string
	CgroupParent    string
	Rootless        bool
	Labels          map[string]string
	DNS             *oci.DNSConfig
	NetworkOpt      netproviders.Opt
	ApparmorProfile string
	Selinux         bool
	ParallelismSem  *semaphore.Weighted
	TraceSocket     string
	Runtime         *RuntimeInfo
}

// NewWorkerOpt creates a WorkerOpt.
func NewWorkerOpt(
	workerOpts WorkerOptions,
	opts ...containerd.ClientOpt,
) (base.WorkerOpt, error) {
	opts = append(opts, containerd.WithDefaultNamespace(workerOpts.Namespace))

	address := workerOpts.Address

	if goRuntime.GOOS == "windows" {
		// TODO(profnandaa): once the upstream PR[1] is merged and
		// vendored in buildkit, we will remove this block.
		// [1] https://github.com/containerd/containerd/pull/9412
		address = strings.TrimPrefix(address, "npipe://")
	}
	client, err := containerd.New(address, opts...)
	if err != nil {
		return base.WorkerOpt{}, errors.Wrapf(err, "failed to connect client to %q . make sure containerd is running", address)
	}
	return newContainerd(
		client,
		workerOpts,
	)
}

func newContainerd(client *containerd.Client, workerOpts WorkerOptions) (base.WorkerOpt, error) {
	if strings.Contains(workerOpts.SnapshotterName, "/") {
		return base.WorkerOpt{}, errors.Errorf("bad snapshotter name: %q", workerOpts.SnapshotterName)
	}
	name := "containerd-" + workerOpts.SnapshotterName
	root := filepath.Join(workerOpts.Root, name)
	if err := os.MkdirAll(root, 0700); err != nil {
		return base.WorkerOpt{}, errors.Wrapf(err, "failed to create %s", root)
	}

	df := client.DiffService()
	// TODO: should use containerd daemon instance ID (containerd/containerd#1862)?
	id, err := base.ID(root)
	if err != nil {
		return base.WorkerOpt{}, err
	}

	serverInfo, err := client.IntrospectionService().Server(context.TODO(), &ptypes.Empty{})
	if err != nil {
		return base.WorkerOpt{}, err
	}

	np, npResolvedMode, err := netproviders.Providers(workerOpts.NetworkOpt)
	if err != nil {
		return base.WorkerOpt{}, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	xlabels := map[string]string{
		wlabel.Executor:       "containerd",
		wlabel.Snapshotter:    workerOpts.SnapshotterName,
		wlabel.Hostname:       hostname,
		wlabel.Network:        npResolvedMode,
		wlabel.SELinuxEnabled: strconv.FormatBool(workerOpts.Selinux),
	}
	if workerOpts.ApparmorProfile != "" {
		xlabels[wlabel.ApparmorProfile] = workerOpts.ApparmorProfile
	}
	xlabels[wlabel.ContainerdNamespace] = workerOpts.Namespace
	xlabels[wlabel.ContainerdUUID] = serverInfo.UUID
	for k, v := range workerOpts.Labels {
		xlabels[k] = v
	}

	lm := leaseutil.WithNamespace(client.LeasesService(), workerOpts.Namespace)

	gc := func(ctx context.Context) (gc.Stats, error) {
		l, err := lm.Create(ctx)
		if err != nil {
			return nil, nil
		}
		return nil, lm.Delete(ctx, leases.Lease{ID: l.ID}, leases.SynchronousDelete)
	}

	cs := containerdsnapshot.NewContentStore(client.ContentStore(), workerOpts.Namespace)

	resp, err := client.IntrospectionService().Plugins(context.TODO(), []string{"type==io.containerd.runtime.v1", "type==io.containerd.runtime.v2"})
	if err != nil {
		return base.WorkerOpt{}, errors.Wrap(err, "failed to list runtime plugin")
	}
	if len(resp.Plugins) == 0 {
		return base.WorkerOpt{}, errors.New("failed to find any runtime plugins")
	}

	var platformSpecs []ocispecs.Platform
	for _, plugin := range resp.Plugins {
		for _, p := range plugin.Platforms {
			// containerd can return platforms that are not normalized
			platformSpecs = append(platformSpecs, platforms.Normalize(ocispecs.Platform{
				OS:           p.OS,
				Architecture: p.Architecture,
				Variant:      p.Variant,
			}))
		}
	}

	snap := containerdsnapshot.NewSnapshotter(workerOpts.SnapshotterName, client.SnapshotService(workerOpts.SnapshotterName), workerOpts.Namespace, nil)

	if err := cache.MigrateV2(
		context.TODO(),
		filepath.Join(root, "metadata.db"),
		filepath.Join(root, "metadata_v2.db"),
		cs,
		snap,
		lm,
	); err != nil {
		return base.WorkerOpt{}, err
	}

	md, err := metadata.NewStore(filepath.Join(root, "metadata_v2.db"))
	if err != nil {
		return base.WorkerOpt{}, err
	}

	executorOpts := containerdexecutor.ExecutorOptions{
		Client:           client,
		Root:             root,
		CgroupParent:     workerOpts.CgroupParent,
		ApparmorProfile:  workerOpts.ApparmorProfile,
		DNSConfig:        workerOpts.DNS,
		Selinux:          workerOpts.Selinux,
		TraceSocket:      workerOpts.TraceSocket,
		Rootless:         workerOpts.Rootless,
		Runtime:          workerOpts.Runtime,
		NetworkProviders: np,
	}

	opt := base.WorkerOpt{
		ID:               id,
		Labels:           xlabels,
		MetadataStore:    md,
		NetworkProviders: np,
		Executor:         containerdexecutor.New(executorOpts),
		Snapshotter:      snap,
		ContentStore:     cs,
		Applier:          winlayers.NewFileSystemApplierWithWindows(cs, df),
		Differ:           winlayers.NewWalkingDiffWithWindows(cs, df),
		ImageStore:       client.ImageService(),
		Platforms:        platformSpecs,
		LeaseManager:     lm,
		GarbageCollect:   gc,
		ParallelismSem:   workerOpts.ParallelismSem,
		MountPoolRoot:    filepath.Join(root, "cachemounts"),
	}
	return opt, nil
}
