package worker

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/rootfs"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver-next"
	"github.com/moby/buildkit/solver-next/llbsolver/ops"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/source/git"
	"github.com/moby/buildkit/source/http"
	"github.com/moby/buildkit/source/local"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TODO: this file should be removed. containerd defines ContainerdWorker, oci defines OCIWorker. There is no base worker.

// WorkerOpt is specific to a worker.
// See also CommonOpt.
type WorkerOpt struct {
	ID             string
	Labels         map[string]string
	SessionManager *session.Manager
	MetadataStore  *metadata.Store
	Executor       executor.Executor
	Snapshotter    snapshot.Snapshotter
	ContentStore   content.Store
	CacheManager   cache.Manager
	ImageSource    source.Source
	Exporters      map[string]exporter.Exporter
	// ImageStore     images.Store // optional
}

// Worker is a local worker instance with dedicated snapshotter, cache, and so on.
// TODO: s/Worker/OpWorker/g ?
type Worker struct {
	WorkerOpt
	SourceManager *source.Manager
	// Exporters     map[string]exporter.Exporter
	// ImageSource source.Source
}

// NewWorker instantiates a local worker
func NewWorker(opt WorkerOpt) (*Worker, error) {
	sm, err := source.NewManager()
	if err != nil {
		return nil, err
	}

	cm := opt.CacheManager
	sm.Register(opt.ImageSource)

	gs, err := git.NewSource(git.Opt{
		CacheAccessor: cm,
		MetadataStore: opt.MetadataStore,
	})
	if err != nil {
		return nil, err
	}

	sm.Register(gs)

	hs, err := http.NewSource(http.Opt{
		CacheAccessor: cm,
		MetadataStore: opt.MetadataStore,
	})
	if err != nil {
		return nil, err
	}

	sm.Register(hs)

	ss, err := local.NewSource(local.Opt{
		SessionManager: opt.SessionManager,
		CacheAccessor:  cm,
		MetadataStore:  opt.MetadataStore,
	})
	if err != nil {
		return nil, err
	}
	sm.Register(ss)

	return &Worker{
		WorkerOpt:     opt,
		SourceManager: sm,
	}, nil
}

func (w *Worker) ID() string {
	return w.WorkerOpt.ID
}

func (w *Worker) Labels() map[string]string {
	return w.WorkerOpt.Labels
}

func (w *Worker) LoadRef(id string) (cache.ImmutableRef, error) {
	return w.CacheManager.Get(context.TODO(), id)
}

func (w *Worker) ResolveOp(v solver.Vertex, s frontend.FrontendLLBBridge) (solver.Op, error) {
	switch op := v.Sys().(type) {
	case *pb.Op_Source:
		return ops.NewSourceOp(v, op, w.SourceManager, w)
	case *pb.Op_Exec:
		return ops.NewExecOp(v, op, w.CacheManager, w.Executor, w)
	case *pb.Op_Build:
		return ops.NewBuildOp(v, op, s, w)
	default:
		return nil, errors.Errorf("could not resolve %v", v)
	}
}

func (w *Worker) ResolveImageConfig(ctx context.Context, ref string) (digest.Digest, []byte, error) {
	// ImageSource is typically source/containerimage
	resolveImageConfig, ok := w.ImageSource.(resolveImageConfig)
	if !ok {
		return "", nil, errors.Errorf("worker %q does not implement ResolveImageConfig", w.ID())
	}
	return resolveImageConfig.ResolveImageConfig(ctx, ref)
}

type resolveImageConfig interface {
	ResolveImageConfig(ctx context.Context, ref string) (digest.Digest, []byte, error)
}

func (w *Worker) Exec(ctx context.Context, meta executor.Meta, rootFS cache.ImmutableRef, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	active, err := w.CacheManager.New(ctx, rootFS)
	if err != nil {
		return err
	}
	defer active.Release(context.TODO())
	return w.Executor.Exec(ctx, meta, active, nil, stdin, stdout, stderr)
}

func (w *Worker) DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error) {
	return w.CacheManager.DiskUsage(ctx, opt)
}

func (w *Worker) Prune(ctx context.Context, ch chan client.UsageInfo) error {
	return w.CacheManager.Prune(ctx, ch)
}

func (w *Worker) Exporter(name string) (exporter.Exporter, error) {
	exp, ok := w.Exporters[name]
	if !ok {
		return nil, errors.Errorf("exporter %q could not be found", name)
	}
	return exp, nil
}

func (w *Worker) GetRemote(ctx context.Context, ref cache.ImmutableRef) (*solver.Remote, error) {
	// diffPairs, err := blobs.GetDiffPairs(ctx, w.ContentStore, w.Snapshotter, w.Differ, ref)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "failed calculaing diff pairs for exported snapshot")
	// }
	// if len(diffPairs) == 0 {
	// 	return nil, nil
	// }
	//
	// descs := make([]ocispec.Descriptor, len(diffPairs))
	//
	// for i, dp := range diffPairs {
	// 	info, err := w.ContentStore.Info(ctx, dp.Blobsum)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	descs[i] = ocispec.Descriptor{
	// 		Digest:    dp.Blobsum,
	// 		Size:      info.Size,
	// 		MediaType: schema2.MediaTypeLayer,
	// 		Annotations: map[string]string{
	// 			"containerd.io/uncompressed": dp.DiffID.String(),
	// 		},
	// 	}
	// }
	//
	// return &solver.Remote{
	// 	Descriptors: descs,
	// 	Provider:    w.ContentStore,
	// }, nil
	return nil, errors.Errorf("getremote not implemented")
}

func (w *Worker) FromRemote(ctx context.Context, remote *solver.Remote) (cache.ImmutableRef, error) {
	// eg, gctx := errgroup.WithContext(ctx)
	// for _, desc := range remote.Descriptors {
	// 	func(desc ocispec.Descriptor) {
	// 		eg.Go(func() error {
	// 			done := oneOffProgress(ctx, fmt.Sprintf("pulling %s", desc.Digest))
	// 			return done(contentutil.Copy(gctx, w.ContentStore, remote.Provider, desc))
	// 		})
	// 	}(desc)
	// }
	//
	// if err := eg.Wait(); err != nil {
	// 	return nil, err
	// }
	//
	// csh, release := snapshot.NewCompatibilitySnapshotter(w.Snapshotter)
	// defer release()
	//
	// unpackProgressDone := oneOffProgress(ctx, "unpacking")
	// chainID, err := w.unpack(ctx, remote.Descriptors, csh)
	// if err != nil {
	// 	return nil, unpackProgressDone(err)
	// }
	// unpackProgressDone(nil)
	//
	// return w.CacheManager.Get(ctx, chainID, cache.WithDescription(fmt.Sprintf("imported %s", remote.Descriptors[len(remote.Descriptors)-1].Digest)))
	return nil, errors.Errorf("fromremote not implemented")
}

// utility function. could be moved to the constructor logic?
// func Labels(executor, snapshotter string) map[string]string {
// 	hostname, err := os.Hostname()
// 	if err != nil {
// 		hostname = "unknown"
// 	}
// 	labels := map[string]string{
// 		worker.LabelOS:          runtime.GOOS,
// 		worker.LabelArch:        runtime.GOOSARCH,
// 		worker.LabelExecutor:    executor,
// 		worker.LabelSnapshotter: snapshotter,
// 		worker.LabelHostname:    hostname,
// 	}
// 	return labels
// }
//
// // ID reads the worker id from the `workerid` file.
// // If not exist, it creates a random one,
// func ID(root string) (string, error) {
// 	f := filepath.Join(root, "workerid")
// 	b, err := ioutil.ReadFile(f)
// 	if err != nil {
// 		if os.IsNotExist(err) {
// 			id := identity.NewID()
// 			err := ioutil.WriteFile(f, []byte(id), 0400)
// 			return id, err
// 		} else {
// 			return "", err
// 		}
// 	}
// 	return string(b), nil
// }

func getLayers(ctx context.Context, descs []ocispec.Descriptor) ([]rootfs.Layer, error) {
	layers := make([]rootfs.Layer, len(descs))
	for i, desc := range descs {
		diffIDStr := desc.Annotations["containerd.io/uncompressed"]
		if diffIDStr == "" {
			return nil, errors.Errorf("%s missing uncompressed digest", desc.Digest)
		}
		diffID, err := digest.Parse(diffIDStr)
		if err != nil {
			return nil, err
		}
		layers[i].Diff = ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    diffID,
		}
		layers[i].Blob = ocispec.Descriptor{
			MediaType: desc.MediaType,
			Digest:    desc.Digest,
			Size:      desc.Size,
		}
	}
	return layers, nil
}

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.FromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	pw.Write(id, st)
	return func(err error) error {
		// TODO: set error on status
		now := time.Now()
		st.Completed = &now
		pw.Write(id, st)
		pw.Close()
		return err
	}
}
