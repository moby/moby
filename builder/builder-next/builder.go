package buildkit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/builder-next/containerimage"
	containerimageexp "github.com/docker/docker/builder/builder-next/exporter"
	"github.com/docker/docker/builder/builder-next/snapshot"
	mobyworker "github.com/docker/docker/builder/builder-next/worker"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/cacheimport"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/dockerfile"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot/blobmapping"
	"github.com/moby/buildkit/solver-next/boltdbcachestorage"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	netcontext "golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	grpcmetadata "google.golang.org/grpc/metadata"
)

// Builder defines interface for running a build
// type Builder interface {
// 	Build(context.Context, backend.BuildConfig) (*builder.Result, error)
// }

// Result is the output produced by a Builder
// type Result struct {
// 	ImageID string
// 	// FromImage Image
// }

type Opt struct {
	SessionManager *session.Manager
	Root           string
	Dist           images.DistributionServices
}

type Builder struct {
	controller *control.Controller
	results    *results

	mu   sync.Mutex
	jobs map[string]func()
}

func New(opt Opt) (*Builder, error) {
	results := newResultsGetter()

	c, err := newController(opt, results.ch)
	if err != nil {
		return nil, err
	}
	b := &Builder{
		controller: c,
		results:    results,
		jobs:       map[string]func(){},
	}
	return b, nil
}

func (b *Builder) Cancel(ctx context.Context, id string) error {
	b.mu.Lock()
	if cancel, ok := b.jobs[id]; ok {
		cancel()
	}
	b.mu.Unlock()
	return nil
}

func (b *Builder) Build(ctx context.Context, opt backend.BuildConfig) (*builder.Result, error) {
	if buildID := opt.Options.BuildID; buildID != "" {
		b.mu.Lock()
		ctx, b.jobs[buildID] = context.WithCancel(ctx)
		b.mu.Unlock()
		defer func() {
			delete(b.jobs, buildID)
		}()
	}

	id := identity.NewID()

	attrs := map[string]string{
		"ref": id,
	}

	frontendAttrs := map[string]string{}

	if opt.Options.Target != "" {
		frontendAttrs["target"] = opt.Options.Target
	}

	if opt.Options.Dockerfile != "" && opt.Options.Dockerfile != "." {
		frontendAttrs["filename"] = opt.Options.Dockerfile
	}

	if opt.Options.RemoteContext != "" {
		frontendAttrs["context"] = opt.Options.RemoteContext
	}

	if len(opt.Options.CacheFrom) > 0 {
		frontendAttrs["cache-from"] = opt.Options.CacheFrom[0]
	}

	logrus.Debugf("frontend: %+v", frontendAttrs)

	for k, v := range opt.Options.BuildArgs {
		if v == nil {
			continue
		}
		frontendAttrs["build-arg:"+k] = *v
	}

	req := &controlapi.SolveRequest{
		Ref:           id,
		Exporter:      "image",
		ExporterAttrs: attrs,
		Frontend:      "dockerfile.v0",
		FrontendAttrs: frontendAttrs,
		Session:       opt.Options.SessionID,
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		_, err := b.controller.Solve(ctx, req)
		return err
	})

	ch := make(chan *controlapi.StatusResponse)

	eg.Go(func() error {
		defer close(ch)
		return b.controller.Status(&controlapi.StatusRequest{
			Ref: id,
		}, &statusProxy{ctx: ctx, ch: ch})
	})

	eg.Go(func() error {
		for sr := range ch {
			dt, err := sr.Marshal()
			if err != nil {
				return err
			}

			auxJSONBytes, err := json.Marshal(dt)
			if err != nil {
				return err
			}
			auxJSON := new(json.RawMessage)
			*auxJSON = auxJSONBytes
			msgJSON, err := json.Marshal(&jsonmessage.JSONMessage{ID: "buildkit-trace", Aux: auxJSON})
			if err != nil {
				return err
			}
			msgJSON = append(msgJSON, []byte("\r\n")...)
			n, err := opt.ProgressWriter.Output.Write(msgJSON)
			if err != nil {
				return err
			}
			if n != len(msgJSON) {
				return io.ErrShortWrite
			}
		}
		return nil
	})

	out := &builder.Result{}
	eg.Go(func() error {
		res, err := b.results.wait(ctx, id)
		if err != nil {
			return err
		}
		out.ImageID = string(res.ID)
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return out, nil
}

func newController(opt Opt, reporter chan containerimageexp.Result) (*control.Controller, error) {
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

	exec, err := runcexecutor.New(runcexecutor.Opt{
		Root:              filepath.Join(root, "executor"),
		CommandCandidates: []string{"docker-runc", "runc"},
	})
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
		Reporter:       reporter,
	})
	if err != nil {
		return nil, err
	}

	cacheStorage, err := boltdbcachestorage.NewStore(filepath.Join(opt.Root, "cache.db"))
	if err != nil {
		return nil, err
	}

	frontends := map[string]frontend.Frontend{}
	frontends["dockerfile.v0"] = dockerfile.NewDockerfileFrontend()
	// frontends["gateway.v0"] = gateway.NewGatewayFrontend()

	// mdb := ctdmetadata.NewDB(db, c, map[string]ctdsnapshot.Snapshotter{
	// 	"moby": s,
	// })
	// if err := mdb.Init(context.TODO()); err != nil {
	// 	return opt, err
	// }
	//
	// throttledGC := throttle.Throttle(time.Second, func() {
	// 	if _, err := mdb.GarbageCollect(context.TODO()); err != nil {
	// 		logrus.Errorf("GC error: %+v", err)
	// 	}
	// })
	//
	// gc := func(ctx context.Context) error {
	// 	throttledGC()
	// 	return nil
	// }

	wopt := mobyworker.WorkerOpt{
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
			"image": exp,
		},
	}

	wc := &worker.Controller{}
	w, err := mobyworker.NewWorker(wopt)
	if err != nil {
		return nil, err
	}
	wc.Add(w)

	ci := cacheimport.NewCacheImporter(cacheimport.ImportOpt{
		Worker:         w,
		SessionManager: opt.SessionManager,
	})

	return control.NewController(control.Opt{
		SessionManager:   opt.SessionManager,
		WorkerController: wc,
		Frontends:        frontends,
		CacheKeyStorage:  cacheStorage,
		// CacheExporter:    ce,
		CacheImporter: ci,
	})
}

type statusProxy struct {
	ctx context.Context
	ch  chan *controlapi.StatusResponse
}

func (sp *statusProxy) SetHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *statusProxy) SendHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *statusProxy) SetTrailer(_ grpcmetadata.MD) {
}

func (sp *statusProxy) Send(resp *controlapi.StatusResponse) error {
	return sp.SendMsg(resp)
}

func (sp *statusProxy) Context() netcontext.Context {
	return sp.ctx
}
func (sp *statusProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.StatusResponse); ok {
		sp.ch <- sr
	}
	return nil
}
func (sp *statusProxy) RecvMsg(m interface{}) error {
	return io.EOF
}

type results struct {
	ch   chan containerimageexp.Result
	res  map[string]containerimageexp.Result
	mu   sync.Mutex
	cond *sync.Cond
}

func newResultsGetter() *results {
	r := &results{
		ch:  make(chan containerimageexp.Result),
		res: map[string]containerimageexp.Result{},
	}
	r.cond = sync.NewCond(&r.mu)

	go func() {
		for res := range r.ch {
			r.mu.Lock()
			r.res[res.Ref] = res
			r.cond.Broadcast()
			r.mu.Unlock()
		}
	}()
	return r
}

func (r *results) wait(ctx context.Context, ref string) (*containerimageexp.Result, error) {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			r.cond.Broadcast()
			r.mu.Unlock()
		case <-done:
		}
	}()

	r.mu.Lock()
	for {
		select {
		case <-ctx.Done():
			r.mu.Unlock()
			return nil, ctx.Err()
		default:
		}
		res, ok := r.res[ref]
		if ok {
			r.mu.Unlock()
			return &res, nil
		}
		r.cond.Wait()
	}
}

type contentStoreNoLabels struct {
	content.Store
}

func (c *contentStoreNoLabels) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, nil
}
