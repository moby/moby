package containerd

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/gc"
	"github.com/containerd/containerd/leases"
	gogoptypes "github.com/gogo/protobuf/types"
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

// NewWorkerOpt creates a WorkerOpt.
func NewWorkerOpt(root string, address, snapshotterName, ns string, rootless bool, labels map[string]string, dns *oci.DNSConfig, nopt netproviders.Opt, apparmorProfile string, selinux bool, parallelismSem *semaphore.Weighted, traceSocket string, opts ...containerd.ClientOpt) (base.WorkerOpt, error) {
	opts = append(opts, containerd.WithDefaultNamespace(ns))
	client, err := containerd.New(address, opts...)
	if err != nil {
		return base.WorkerOpt{}, errors.Wrapf(err, "failed to connect client to %q . make sure containerd is running", address)
	}
	return newContainerd(root, client, snapshotterName, ns, rootless, labels, dns, nopt, apparmorProfile, selinux, parallelismSem, traceSocket)
}

func newContainerd(root string, client *containerd.Client, snapshotterName, ns string, rootless bool, labels map[string]string, dns *oci.DNSConfig, nopt netproviders.Opt, apparmorProfile string, selinux bool, parallelismSem *semaphore.Weighted, traceSocket string) (base.WorkerOpt, error) {
	if strings.Contains(snapshotterName, "/") {
		return base.WorkerOpt{}, errors.Errorf("bad snapshotter name: %q", snapshotterName)
	}
	name := "containerd-" + snapshotterName
	root = filepath.Join(root, name)
	if err := os.MkdirAll(root, 0700); err != nil {
		return base.WorkerOpt{}, errors.Wrapf(err, "failed to create %s", root)
	}

	df := client.DiffService()
	// TODO: should use containerd daemon instance ID (containerd/containerd#1862)?
	id, err := base.ID(root)
	if err != nil {
		return base.WorkerOpt{}, err
	}

	serverInfo, err := client.IntrospectionService().Server(context.TODO(), &gogoptypes.Empty{})
	if err != nil {
		return base.WorkerOpt{}, err
	}

	np, npResolvedMode, err := netproviders.Providers(nopt)
	if err != nil {
		return base.WorkerOpt{}, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	xlabels := map[string]string{
		wlabel.Executor:       "containerd",
		wlabel.Snapshotter:    snapshotterName,
		wlabel.Hostname:       hostname,
		wlabel.Network:        npResolvedMode,
		wlabel.SELinuxEnabled: strconv.FormatBool(selinux),
	}
	if apparmorProfile != "" {
		xlabels[wlabel.ApparmorProfile] = apparmorProfile
	}
	xlabels[wlabel.ContainerdNamespace] = ns
	xlabels[wlabel.ContainerdUUID] = serverInfo.UUID
	for k, v := range labels {
		xlabels[k] = v
	}

	lm := leaseutil.WithNamespace(client.LeasesService(), ns)

	gc := func(ctx context.Context) (gc.Stats, error) {
		l, err := lm.Create(ctx)
		if err != nil {
			return nil, nil
		}
		return nil, lm.Delete(ctx, leases.Lease{ID: l.ID}, leases.SynchronousDelete)
	}

	cs := containerdsnapshot.NewContentStore(client.ContentStore(), ns)

	resp, err := client.IntrospectionService().Plugins(context.TODO(), []string{"type==io.containerd.runtime.v1", "type==io.containerd.runtime.v2"})
	if err != nil {
		return base.WorkerOpt{}, errors.Wrap(err, "failed to list runtime plugin")
	}
	if len(resp.Plugins) == 0 {
		return base.WorkerOpt{}, errors.New("failed to find any runtime plugins")
	}

	var platforms []ocispecs.Platform
	for _, plugin := range resp.Plugins {
		for _, p := range plugin.Platforms {
			platforms = append(platforms, ocispecs.Platform{
				OS:           p.OS,
				Architecture: p.Architecture,
				Variant:      p.Variant,
			})
		}
	}

	snap := containerdsnapshot.NewSnapshotter(snapshotterName, client.SnapshotService(snapshotterName), ns, nil)

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

	opt := base.WorkerOpt{
		ID:               id,
		Labels:           xlabels,
		MetadataStore:    md,
		NetworkProviders: np,
		Executor:         containerdexecutor.New(client, root, "", np, dns, apparmorProfile, selinux, traceSocket, rootless),
		Snapshotter:      snap,
		ContentStore:     cs,
		Applier:          winlayers.NewFileSystemApplierWithWindows(cs, df),
		Differ:           winlayers.NewWalkingDiffWithWindows(cs, df),
		ImageStore:       client.ImageService(),
		Platforms:        platforms,
		LeaseManager:     lm,
		GarbageCollect:   gc,
		ParallelismSem:   parallelismSem,
		MountPoolRoot:    filepath.Join(root, "cachemounts"),
	}
	return opt, nil
}
