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
	"github.com/docker/docker/builder/builder-next/exporter/mobyexporter"
	"github.com/docker/docker/builder/builder-next/imagerefchecker"
	mobyworker "github.com/docker/docker/builder/builder-next/worker"
	wlabel "github.com/docker/docker/builder/builder-next/worker/label"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/graphdriver"
	units "github.com/docker/go-units"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/cache/remotecache/gha"
	inlineremotecache "github.com/moby/buildkit/cache/remotecache/inline"
	localremotecache "github.com/moby/buildkit/cache/remotecache/local"
	registryremotecache "github.com/moby/buildkit/cache/remotecache/registry"
	"github.com/moby/buildkit/client"
	bkconfig "github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/frontend"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/frontend/gateway/forwarder"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/solver/bboltcachestorage"
	"github.com/moby/buildkit/util/archutil"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/containerd"
	"github.com/moby/buildkit/worker/label"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
	bolt "go.etcd.io/bbolt"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
)

func newController(ctx context.Context, rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if opt.UseSnapshotter {
		return newSnapshotterController(ctx, rt, opt)
	}
	return newGraphDriverController(ctx, rt, opt)
}

func newSnapshotterController(ctx context.Context, rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0o711); err != nil {
		return nil, err
	}

	historyDB, historyConf, err := openHistoryDB(opt.Root, opt.BuilderConfig.History)
	if err != nil {
		return nil, err
	}

	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	nc := netproviders.Opt{
		Mode: "host",
	}
	dns := getDNSConfig(opt.DNSConfig)

	wo, err := containerd.NewWorkerOpt(opt.Root, opt.ContainerdAddress, opt.Snapshotter, opt.ContainerdNamespace,
		opt.Rootless, map[string]string{
			label.Snapshotter: opt.Snapshotter,
		}, dns, nc, opt.ApparmorProfile, false, nil, "", ctd.WithTimeout(60*time.Second))
	if err != nil {
		return nil, err
	}

	policy, err := getGCPolicy(opt.BuilderConfig, opt.Root)
	if err != nil {
		return nil, err
	}

	wo.GCPolicy = policy
	wo.RegistryHosts = opt.RegistryHosts
	wo.Labels = getLabels(opt, wo.Labels)

	exec, err := newExecutor(opt.Root, opt.DefaultCgroupParent, opt.NetworkController, dns, opt.Rootless, opt.IdentityMapping, opt.ApparmorProfile)
	if err != nil {
		return nil, err
	}
	wo.Executor = exec

	w, err := mobyworker.NewContainerdWorker(ctx, wo)
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

	return control.NewController(control.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheKeyStorage:  cacheStorage,
		ResolveCacheImporterFuncs: map[string]remotecache.ResolveCacheImporterFunc{
			"gha":      gha.ResolveCacheImporterFunc(),
			"local":    localremotecache.ResolveCacheImporterFunc(opt.SessionManager),
			"registry": registryremotecache.ResolveCacheImporterFunc(opt.SessionManager, wo.ContentStore, opt.RegistryHosts),
		},
		ResolveCacheExporterFuncs: map[string]remotecache.ResolveCacheExporterFunc{
			"gha":      gha.ResolveCacheExporterFunc(),
			"inline":   inlineremotecache.ResolveCacheExporterFunc(),
			"local":    localremotecache.ResolveCacheExporterFunc(opt.SessionManager),
			"registry": registryremotecache.ResolveCacheExporterFunc(opt.SessionManager, opt.RegistryHosts),
		},
		Entitlements:  getEntitlements(opt.BuilderConfig),
		HistoryDB:     historyDB,
		HistoryConfig: historyConf,
		LeaseManager:  wo.LeaseManager,
		ContentStore:  wo.ContentStore,
	})
}

func openHistoryDB(root string, cfg *config.BuilderHistoryConfig) (*bolt.DB, *bkconfig.HistoryConfig, error) {
	db, err := bbolt.Open(filepath.Join(root, "history.db"), 0o600, nil)
	if err != nil {
		return nil, nil, err
	}

	var conf *bkconfig.HistoryConfig
	if cfg != nil {
		conf = &bkconfig.HistoryConfig{
			MaxAge:     cfg.MaxAge,
			MaxEntries: cfg.MaxEntries,
		}
	}

	return db, conf, nil
}

func newGraphDriverController(ctx context.Context, rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0711); err != nil {
		return nil, err
	}

	dist := opt.Dist
	root := opt.Root

	pb.Caps.Init(apicaps.Cap{
		ID:                pb.CapMergeOp,
		Enabled:           false,
		DisabledReasonMsg: "only enabled with containerd image store backend",
	})

	pb.Caps.Init(apicaps.Cap{
		ID:                pb.CapDiffOp,
		Enabled:           false,
		DisabledReasonMsg: "only enabled with containerd image store backend",
	})

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

	differ, ok := snapshotter.(mobyexporter.Differ)
	if !ok {
		return nil, errors.Errorf("snapshotter doesn't support differ")
	}

	exp, err := mobyexporter.New(mobyexporter.Opt{
		ImageStore:  dist.ImageStore,
		Differ:      differ,
		ImageTagger: opt.ImageTagger,
	})
	if err != nil {
		return nil, err
	}

	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	historyDB, historyConf, err := openHistoryDB(opt.Root, opt.BuilderConfig.History)
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

	leases, err := lm.List(ctx, "labels.\"buildkit/lease.temporary\"")
	if err != nil {
		return nil, err
	}
	for _, l := range leases {
		lm.Delete(ctx, l)
	}

	wopt := mobyworker.Opt{
		ID:                opt.EngineID,
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
		LeaseManager:      lm,
		Labels:            getLabels(opt, nil),
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

	return control.NewController(control.Opt{
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
		Entitlements:  getEntitlements(opt.BuilderConfig),
		LeaseManager:  lm,
		ContentStore:  store,
		HistoryDB:     historyDB,
		HistoryConfig: historyConf,
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

func getLabels(opt Opt, labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[wlabel.HostGatewayIP] = opt.DNSConfig.HostGatewayIP.String()
	return labels
}
