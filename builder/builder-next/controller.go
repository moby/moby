package buildkit

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/content/local"
	"github.com/docker/docker/builder/builder-next/adapters/containerimage"
	"github.com/docker/docker/builder/builder-next/adapters/snapshot"
	containerimageexp "github.com/docker/docker/builder/builder-next/exporter"
	mobyworker "github.com/docker/docker/builder/builder-next/worker"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	registryremotecache "github.com/moby/buildkit/cache/remotecache/registry"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/frontend/gateway/forwarder"
	"github.com/moby/buildkit/snapshot/blobmapping"
	"github.com/moby/buildkit/solver/boltdbcachestorage"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
)

func newController(rt http.RoundTripper, opt Opt) (*control.Controller, error) {
	if err := os.MkdirAll(opt.Root, 0700); err != nil {
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

	sbase, err := snapshot.NewSnapshotter(snapshot.Opt{
		GraphDriver: driver,
		LayerStore:  dist.LayerStore,
		Root:        root,
	})
	if err != nil {
		return nil, err
	}

	store, err := local.NewStore(filepath.Join(root, "content"))
	if err != nil {
		return nil, err
	}
	store = &contentStoreNoLabels{store}

	md, err := metadata.NewStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return nil, err
	}

	snapshotter := blobmapping.NewSnapshotter(blobmapping.Opt{
		Content:       store,
		Snapshotter:   sbase,
		MetadataStore: md,
	})

	cm, err := cache.NewManager(cache.ManagerOpt{
		Snapshotter:   snapshotter,
		MetadataStore: md,
		// TODO: implement PruneRefChecker to correctly mark cache objects as "Shared"
		PruneRefChecker: nil,
	})
	if err != nil {
		return nil, err
	}

	src, err := containerimage.NewSource(containerimage.SourceOpt{
		SessionManager:  opt.SessionManager,
		CacheAccessor:   cm,
		ContentStore:    store,
		DownloadManager: dist.DownloadManager,
		MetadataStore:   dist.V2MetadataService,
		ImageStore:      dist.ImageStore,
		ReferenceStore:  dist.ReferenceStore,
	})
	if err != nil {
		return nil, err
	}

	exec, err := newExecutor(root, opt.NetnsRoot, opt.NetworkController)
	if err != nil {
		return nil, err
	}

	differ, ok := sbase.(containerimageexp.Differ)
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

	cacheStorage, err := boltdbcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	wopt := mobyworker.Opt{
		ID:                "moby",
		SessionManager:    opt.SessionManager,
		MetadataStore:     md,
		ContentStore:      store,
		CacheManager:      cm,
		Snapshotter:       snapshotter,
		Executor:          exec,
		ImageSource:       src,
		DownloadManager:   dist.DownloadManager,
		V2MetadataService: dist.V2MetadataService,
		Exporters: map[string]exporter.Exporter{
			"moby": exp,
		},
		Transport: rt,
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
		SessionManager:           opt.SessionManager,
		WorkerController:         wc,
		Frontends:                frontends,
		CacheKeyStorage:          cacheStorage,
		ResolveCacheImporterFunc: registryremotecache.ResolveCacheImporterFunc(opt.SessionManager),
		// TODO: set ResolveCacheExporterFunc for exporting cache
	})
}
