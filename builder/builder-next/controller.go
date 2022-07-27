package buildkit

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ctd "github.com/containerd/containerd"
	"github.com/containerd/containerd/content/local"
	ctdmetadata "github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/builder/builder-next/adapters/containerimage"
	"github.com/docker/docker/builder/builder-next/adapters/localinlinecache"
	"github.com/docker/docker/builder/builder-next/adapters/snapshot"
	mobycontrol "github.com/docker/docker/builder/builder-next/control"
	containerimageexp "github.com/docker/docker/builder/builder-next/exporter"
	"github.com/docker/docker/builder/builder-next/imagerefchecker"
	mobyworker "github.com/docker/docker/builder/builder-next/worker"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/graphdriver"
	units "github.com/docker/go-units"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/cache/remotecache"
	inlineremotecache "github.com/moby/buildkit/cache/remotecache/inline"
	localremotecache "github.com/moby/buildkit/cache/remotecache/local"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/frontend"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/frontend/gateway/forwarder"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/solver/bboltcachestorage"
	"github.com/moby/buildkit/util/archutil"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/network/cniprovider"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/containerd"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

func newController(rt http.RoundTripper, opt Opt) (*mobycontrol.Controller, error) {
	if opt.UseSnapshotter {
		return newSnapshotterController(rt, opt)
	}
	return newGrapDriverController(rt, opt)
}

func newSnapshotterController(rt http.RoundTripper, opt Opt) (*mobycontrol.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0711); err != nil {
		return nil, err
	}

	dist := opt.Dist

	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	nc := netproviders.Opt{
		Mode: "auto",
		CNI: cniprovider.Opt{
			Root:       opt.Root,
			ConfigPath: "/etc/buildkit/cni.json",
			BinaryDir:  "/opt/cni/bin",
		},
	}
	dns := getDNSConfig(opt.DNSConfig)

	snapshotter := ctd.DefaultSnapshotter

	wo, err := containerd.NewWorkerOpt(opt.Root, opt.ContainerdAddress, snapshotter, opt.ContainerdNamespace,
		opt.Rootless, map[string]string{}, dns, nc, opt.ApparmorProfile, nil, "", ctd.WithTimeout(60*time.Second))
	if err != nil {
		return nil, err
	}

	policy, err := getGCPolicy(opt.BuilderConfig, opt.Root)
	if err != nil {
		return nil, err
	}

	wo.GCPolicy = policy
	wo.RegistryHosts = opt.RegistryHosts

	w, err := mobyworker.NewContainerdWorker(context.TODO(), wo)
	if err != nil {
		return nil, err
	}

	wc := &worker.Controller{}

	err = wc.Add(w)
	if err != nil {
		return nil, err
	}
	frontends := map[string]frontend.Frontend{
		"dockerfile.v0": forwarder.NewGatewayForwarder(wc, dockerfile.Build),
		"gateway.v0":    gateway.NewGatewayFrontend(wc),
	}

	wa, err := wc.GetDefault()
	if err != nil {
		return nil, err
	}

	return mobycontrol.NewController(mobycontrol.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheKeyStorage:  cacheStorage,
		ResolveCacheImporterFuncs: map[string]remotecache.ResolveCacheImporterFunc{
			"registry": localinlinecache.ResolveCacheImporterFunc(opt.SessionManager, opt.RegistryHosts, wa.ContentStore(), dist.ReferenceStore, dist.ImageStore),
			"local":    localremotecache.ResolveCacheImporterFunc(opt.SessionManager),
		},
		ResolveCacheExporterFuncs: map[string]remotecache.ResolveCacheExporterFunc{
			"inline": inlineremotecache.ResolveCacheExporterFunc(),
		},
		Entitlements:   getEntitlements(opt.BuilderConfig),
		UseSnapshotter: true,
	})
}

func newGrapDriverController(rt http.RoundTripper, opt Opt) (*mobycontrol.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0711); err != nil {
		return nil, err
	}

	dist := opt.Dist
	root := opt.Root

	var driver graphdriver.Driver
	if ls, ok := dist.LayerStore.(interface {
		Driver() graphdriver.Driver
	}); ok {
		driver = ls.Driver()
	} else {
		return nil, errors.Errorf("could not access graphdriver")
	}

	store, err := local.NewStore(filepath.Join(root, "content"))
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(filepath.Join(root, "containerdmeta.db"), 0644, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mdb := ctdmetadata.NewDB(db, store, map[string]snapshots.Snapshotter{})

	store = containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit")

	lm := leaseutil.WithNamespace(ctdmetadata.NewLeaseManager(mdb), "buildkit")

	snapshotter, lm, err := snapshot.NewSnapshotter(snapshot.Opt{
		GraphDriver:     driver,
		LayerStore:      dist.LayerStore,
		Root:            root,
		IdentityMapping: opt.IdentityMapping,
	}, lm)
	if err != nil {
		return nil, err
	}

	if err := cache.MigrateV2(context.Background(), filepath.Join(root, "metadata.db"), filepath.Join(root, "metadata_v2.db"), store, snapshotter, lm); err != nil {
		return nil, err
	}

	md, err := metadata.NewStore(filepath.Join(root, "metadata_v2.db"))
	if err != nil {
		return nil, err
	}

	layerGetter, ok := snapshotter.(imagerefchecker.LayerGetter)
	if !ok {
		return nil, errors.Errorf("snapshotter does not implement layergetter")
	}

	refChecker := imagerefchecker.New(imagerefchecker.Opt{
		ImageStore:  dist.ImageStore,
		LayerGetter: layerGetter,
	})

	cm, err := cache.NewManager(cache.ManagerOpt{
		Snapshotter:     snapshotter,
		MetadataStore:   md,
		PruneRefChecker: refChecker,
		LeaseManager:    lm,
		ContentStore:    store,
		GarbageCollect:  mdb.GarbageCollect,
	})
	if err != nil {
		return nil, err
	}

	src, err := containerimage.NewSource(containerimage.SourceOpt{
		CacheAccessor:   cm,
		ContentStore:    store,
		DownloadManager: dist.DownloadManager,
		MetadataStore:   dist.V2MetadataService,
		ImageStore:      dist.ImageStore,
		ReferenceStore:  dist.ReferenceStore,
		RegistryHosts:   opt.RegistryHosts,
		LayerStore:      dist.LayerStore,
		LeaseManager:    lm,
		GarbageCollect:  mdb.GarbageCollect,
	})
	if err != nil {
		return nil, err
	}

	dns := getDNSConfig(opt.DNSConfig)

	exec, err := newExecutor(root, opt.DefaultCgroupParent, opt.NetworkController, dns, opt.Rootless, opt.IdentityMapping, opt.ApparmorProfile)
	if err != nil {
		return nil, err
	}

	differ, ok := snapshotter.(containerimageexp.Differ)
	if !ok {
		return nil, errors.Errorf("snapshotter doesn't support differ")
	}

	exp, err := containerimageexp.New(containerimageexp.Opt{
		ImageStore:     dist.ImageStore,
		ReferenceStore: dist.ReferenceStore,
		Differ:         differ,
	})
	if err != nil {
		return nil, err
	}

	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	gcPolicy, err := getGCPolicy(opt.BuilderConfig, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not get builder GC policy")
	}

	layers, ok := snapshotter.(mobyworker.LayerAccess)
	if !ok {
		return nil, errors.Errorf("snapshotter doesn't support differ")
	}

	leases, err := lm.List(context.TODO(), "labels.\"buildkit/lease.temporary\"")
	if err != nil {
		return nil, err
	}
	for _, l := range leases {
		lm.Delete(context.TODO(), l)
	}

	wopt := mobyworker.Opt{
		ID:                "moby",
		ContentStore:      store,
		CacheManager:      cm,
		GCPolicy:          gcPolicy,
		Snapshotter:       snapshotter,
		Executor:          exec,
		ImageSource:       src,
		DownloadManager:   dist.DownloadManager,
		V2MetadataService: dist.V2MetadataService,
		Exporter:          exp,
		Transport:         rt,
		Layers:            layers,
		Platforms:         archutil.SupportedPlatforms(true),
	}

	wc := &worker.Controller{}
	w, err := mobyworker.NewWorker(wopt)
	if err != nil {
		return nil, err
	}
	wc.Add(w)

	frontends := map[string]frontend.Frontend{
		"dockerfile.v0": forwarder.NewGatewayForwarder(wc, dockerfile.Build),
		"gateway.v0":    gateway.NewGatewayFrontend(wc),
	}

	return mobycontrol.NewController(mobycontrol.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheKeyStorage:  cacheStorage,
		ResolveCacheImporterFuncs: map[string]remotecache.ResolveCacheImporterFunc{
			"registry": localinlinecache.ResolveCacheImporterFunc(opt.SessionManager, opt.RegistryHosts, store, dist.ReferenceStore, dist.ImageStore),
			"local":    localremotecache.ResolveCacheImporterFunc(opt.SessionManager),
		},
		ResolveCacheExporterFuncs: map[string]remotecache.ResolveCacheExporterFunc{
			"inline": inlineremotecache.ResolveCacheExporterFunc(),
		},
		Entitlements:   getEntitlements(opt.BuilderConfig),
		UseSnapshotter: false,
	})
}

func getGCPolicy(conf config.BuilderConfig, root string) ([]client.PruneInfo, error) {
	var gcPolicy []client.PruneInfo
	if conf.GC.Enabled {
		var (
			defaultKeepStorage int64
			err                error
		)

		if conf.GC.DefaultKeepStorage != "" {
			defaultKeepStorage, err = units.RAMInBytes(conf.GC.DefaultKeepStorage)
			if err != nil {
				return nil, errors.Wrapf(err, "could not parse '%s' as Builder.GC.DefaultKeepStorage config", conf.GC.DefaultKeepStorage)
			}
		}

		if conf.GC.Policy == nil {
			gcPolicy = mobyworker.DefaultGCPolicy(root, defaultKeepStorage)
		} else {
			gcPolicy = make([]client.PruneInfo, len(conf.GC.Policy))
			for i, p := range conf.GC.Policy {
				b, err := units.RAMInBytes(p.KeepStorage)
				if err != nil {
					return nil, err
				}
				if b == 0 {
					b = defaultKeepStorage
				}
				gcPolicy[i], err = toBuildkitPruneInfo(types.BuildCachePruneOptions{
					All:         p.All,
					KeepStorage: b,
					Filters:     filters.Args(p.Filter),
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return gcPolicy, nil
}

func getEntitlements(conf config.BuilderConfig) []string {
	var ents []string
	// Incase of no config settings, NetworkHost should be enabled & SecurityInsecure must be disabled.
	if conf.Entitlements.NetworkHost == nil || *conf.Entitlements.NetworkHost {
		ents = append(ents, string(entitlements.EntitlementNetworkHost))
	}
	if conf.Entitlements.SecurityInsecure != nil && *conf.Entitlements.SecurityInsecure {
		ents = append(ents, string(entitlements.EntitlementSecurityInsecure))
	}
	return ents
}
