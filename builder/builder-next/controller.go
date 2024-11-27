package buildkit

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	ctd "github.com/containerd/containerd"
	"github.com/containerd/containerd/content/local"
	ctdmetadata "github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
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
	"github.com/docker/go-units"
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
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/bboltcachestorage"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/util/archutil"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/containerd"
	"github.com/moby/buildkit/worker/label"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
	bolt "go.etcd.io/bbolt"
	"go.opentelemetry.io/otel/sdk/trace"
)

func newController(ctx context.Context, rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if opt.UseSnapshotter {
		return newSnapshotterController(ctx, rt, opt)
	}
	return newGraphDriverController(ctx, rt, opt)
}

func getTraceExporter(ctx context.Context) trace.SpanExporter {
	tc := make(tracing.MultiSpanExporter, 0, 2)
	if detect.Recorder != nil {
		tc = append(tc, detect.Recorder)
	}

	if exp, err := detect.NewSpanExporter(ctx); err != nil {
		log.G(ctx).WithError(err).Error("Failed to detect trace exporter for buildkit controller")
	} else if !detect.IsNoneSpanExporter(exp) {
		tc = append(tc, exp)
	}
	return tc
}

func newSnapshotterController(ctx context.Context, rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0o711); err != nil {
		return nil, err
	}

	historyDB, historyConf, err := openHistoryDB(opt.Root, "history_c8d.db", opt.BuilderConfig.History)
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

	// HACK! Windows doesn't have 'host' mode networking.
	if runtime.GOOS == "windows" {
		nc = netproviders.Opt{
			Mode: "auto",
		}
	}

	dns := getDNSConfig(opt.DNSConfig)

	workerOpts := containerd.WorkerOptions{
		Root:            opt.Root,
		Address:         opt.ContainerdAddress,
		SnapshotterName: opt.Snapshotter,
		Namespace:       opt.ContainerdNamespace,
		Rootless:        opt.Rootless,
		Labels: map[string]string{
			label.Snapshotter: opt.Snapshotter,
		},
		DNS:             dns,
		NetworkOpt:      nc,
		ApparmorProfile: opt.ApparmorProfile,
		Selinux:         false,
	}

	wo, err := containerd.NewWorkerOpt(workerOpts, ctd.WithTimeout(60*time.Second))
	if err != nil {
		return nil, err
	}

	policy, err := getGCPolicy(opt.BuilderConfig, opt.Root)
	if err != nil {
		return nil, err
	}

	// make sure platforms are normalized moby/buildkit#4391
	for i, p := range wo.Platforms {
		wo.Platforms[i] = platforms.Normalize(p)
	}

	wo.GCPolicy = policy
	wo.RegistryHosts = opt.RegistryHosts
	wo.Labels = getLabels(opt, wo.Labels)

	exec, err := newExecutor(opt.Root, opt.DefaultCgroupParent, opt.NetworkController, dns, opt.Rootless, opt.IdentityMapping, opt.ApparmorProfile)
	if err != nil {
		return nil, err
	}
	wo.Executor = exec

	w, err := mobyworker.NewContainerdWorker(ctx, wo, opt.Callbacks)
	if err != nil {
		return nil, err
	}

	wc := &worker.Controller{}

	err = wc.Add(w)
	if err != nil {
		return nil, err
	}

	gwf, err := gateway.NewGatewayFrontend(wc.Infos(), nil)
	if err != nil {
		return nil, err
	}

	frontends := map[string]frontend.Frontend{
		"dockerfile.v0": forwarder.NewGatewayForwarder(wc.Infos(), dockerfile.Build),
		"gateway.v0":    gwf,
	}

	return control.NewController(control.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheManager:     solver.NewCacheManager(ctx, "local", cacheStorage, worker.NewCacheResultStorage(wc)),
		CacheStore:       cacheStorage,
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
		Entitlements:   getEntitlements(opt.BuilderConfig),
		HistoryDB:      historyDB,
		HistoryConfig:  historyConf,
		LeaseManager:   wo.LeaseManager,
		ContentStore:   wo.ContentStore,
		TraceCollector: getTraceExporter(ctx),
		GarbageCollect: w.GarbageCollect,
	})
}

func openHistoryDB(root string, fn string, cfg *config.BuilderHistoryConfig) (*bolt.DB, *bkconfig.HistoryConfig, error) {
	db, err := bbolt.Open(filepath.Join(root, fn), 0o600, nil)
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
	if err := os.MkdirAll(opt.Root, 0o711); err != nil {
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

	innerStore, err := local.NewStore(filepath.Join(root, "content"))
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(filepath.Join(root, "containerdmeta.db"), 0o644, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mdb := ctdmetadata.NewDB(db, innerStore, map[string]snapshots.Snapshotter{})

	store := containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit")

	snapshotter, lm, err := snapshot.NewSnapshotter(snapshot.Opt{
		GraphDriver:     driver,
		LayerStore:      dist.LayerStore,
		Root:            root,
		IdentityMapping: opt.IdentityMapping,
	}, ctdmetadata.NewLeaseManager(mdb), "buildkit")
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
		ImageStore:            dist.ImageStore,
		ContentStore:          store,
		Differ:                differ,
		ImageTagger:           opt.ImageTagger,
		LeaseManager:          lm,
		ImageExportedCallback: opt.Callbacks.Exported,
		// Callbacks.Named is not used here because the tag operation is handled directly by the image service.
	})
	if err != nil {
		return nil, err
	}

	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	historyDB, historyConf, err := openHistoryDB(opt.Root, "history.db", opt.BuilderConfig.History)
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

	leases, err := lm.List(ctx, `labels."buildkit/lease.temporary"`)
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
		GarbageCollect:    mdb.GarbageCollect,
		Labels:            getLabels(opt, nil),
	}

	wc := &worker.Controller{}
	w, err := mobyworker.NewWorker(wopt)
	if err != nil {
		return nil, err
	}
	wc.Add(w)

	gwf, err := gateway.NewGatewayFrontend(wc.Infos(), nil)
	if err != nil {
		return nil, err
	}

	frontends := map[string]frontend.Frontend{
		"dockerfile.v0": forwarder.NewGatewayForwarder(wc.Infos(), dockerfile.Build),
		"gateway.v0":    gwf,
	}

	return control.NewController(control.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheManager:     solver.NewCacheManager(ctx, "local", cacheStorage, worker.NewCacheResultStorage(wc)),
		CacheStore:       cacheStorage,
		ResolveCacheImporterFuncs: map[string]remotecache.ResolveCacheImporterFunc{
			"registry": localinlinecache.ResolveCacheImporterFunc(opt.SessionManager, opt.RegistryHosts, store, dist.ReferenceStore, dist.ImageStore),
			"local":    localremotecache.ResolveCacheImporterFunc(opt.SessionManager),
		},
		ResolveCacheExporterFuncs: map[string]remotecache.ResolveCacheExporterFunc{
			"inline": inlineremotecache.ResolveCacheExporterFunc(),
		},
		Entitlements:   getEntitlements(opt.BuilderConfig),
		LeaseManager:   lm,
		ContentStore:   store,
		HistoryDB:      historyDB,
		HistoryConfig:  historyConf,
		TraceCollector: getTraceExporter(ctx),
		GarbageCollect: w.GarbageCollect,
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

				// FIXME(thaJeztah): wire up new options https://github.com/moby/moby/issues/48639
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
	if len(opt.DNSConfig.HostGatewayIPs) > 0 {
		// TODO(robmry) - buildx has its own version of toBuildkitExtraHosts(), which
		//   needs to be updated to understand >1 address. For now, take the IPv4 address
		//   if there is one, else IPv6.
		for _, gip := range opt.DNSConfig.HostGatewayIPs {
			labels[wlabel.HostGatewayIP] = gip.String()
			if gip.Is4() {
				break
			}
		}
	}
	return labels
}
