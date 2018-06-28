package worker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	nethttp "net/http"
	"runtime"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/rootfs"
	"github.com/docker/docker/distribution"
	distmetadata "github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	pkgprogress "github.com/docker/docker/pkg/progress"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver/ops"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/source/git"
	"github.com/moby/buildkit/source/http"
	"github.com/moby/buildkit/source/local"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Opt defines a structure for creating a worker.
type Opt struct {
	ID                string
	Labels            map[string]string
	SessionManager    *session.Manager
	MetadataStore     *metadata.Store
	Executor          executor.Executor
	Snapshotter       snapshot.Snapshotter
	ContentStore      content.Store
	CacheManager      cache.Manager
	ImageSource       source.Source
	Exporters         map[string]exporter.Exporter
	DownloadManager   distribution.RootFSDownloadManager
	V2MetadataService distmetadata.V2MetadataService
	Transport         nethttp.RoundTripper
}

// Worker is a local worker instance with dedicated snapshotter, cache, and so on.
// TODO: s/Worker/OpWorker/g ?
type Worker struct {
	Opt
	SourceManager *source.Manager
}

// NewWorker instantiates a local worker
func NewWorker(opt Opt) (*Worker, error) {
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
	if err == nil {
		sm.Register(gs)
	} else {
		logrus.Warnf("Could not register builder git source: %s", err)
	}

	hs, err := http.NewSource(http.Opt{
		CacheAccessor: cm,
		MetadataStore: opt.MetadataStore,
		Transport:     opt.Transport,
	})
	if err == nil {
		sm.Register(hs)
	} else {
		logrus.Warnf("Could not register builder http source: %s", err)
	}

	ss, err := local.NewSource(local.Opt{
		SessionManager: opt.SessionManager,
		CacheAccessor:  cm,
		MetadataStore:  opt.MetadataStore,
	})
	if err == nil {
		sm.Register(ss)
	} else {
		logrus.Warnf("Could not register builder local source: %s", err)
	}

	return &Worker{
		Opt:           opt,
		SourceManager: sm,
	}, nil
}

// ID returns worker ID
func (w *Worker) ID() string {
	return w.Opt.ID
}

// Labels returns map of all worker labels
func (w *Worker) Labels() map[string]string {
	return w.Opt.Labels
}

// Platforms returns one or more platforms supported by the image.
func (w *Worker) Platforms() []ocispec.Platform {
	// does not handle lcow
	return []ocispec.Platform{platforms.DefaultSpec()}
}

// LoadRef loads a reference by ID
func (w *Worker) LoadRef(id string) (cache.ImmutableRef, error) {
	return w.CacheManager.Get(context.TODO(), id)
}

// ResolveOp converts a LLB vertex into a LLB operation
func (w *Worker) ResolveOp(v solver.Vertex, s frontend.FrontendLLBBridge) (solver.Op, error) {
	if baseOp, ok := v.Sys().(*pb.Op); ok {
		switch op := baseOp.Op.(type) {
		case *pb.Op_Source:
			return ops.NewSourceOp(v, op, baseOp.Platform, w.SourceManager, w)
		case *pb.Op_Exec:
			return ops.NewExecOp(v, op, w.CacheManager, w.MetadataStore, w.Executor, w)
		case *pb.Op_Build:
			return ops.NewBuildOp(v, op, s, w)
		}
	}
	return nil, errors.Errorf("could not resolve %v", v)
}

// ResolveImageConfig returns image config for an image
func (w *Worker) ResolveImageConfig(ctx context.Context, ref string, platform *ocispec.Platform) (digest.Digest, []byte, error) {
	// ImageSource is typically source/containerimage
	resolveImageConfig, ok := w.ImageSource.(resolveImageConfig)
	if !ok {
		return "", nil, errors.Errorf("worker %q does not implement ResolveImageConfig", w.ID())
	}
	return resolveImageConfig.ResolveImageConfig(ctx, ref, platform)
}

// Exec executes a process directly on a worker
func (w *Worker) Exec(ctx context.Context, meta executor.Meta, rootFS cache.ImmutableRef, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	active, err := w.CacheManager.New(ctx, rootFS)
	if err != nil {
		return err
	}
	defer active.Release(context.TODO())
	return w.Executor.Exec(ctx, meta, active, nil, stdin, stdout, stderr)
}

// DiskUsage returns disk usage report
func (w *Worker) DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error) {
	return w.CacheManager.DiskUsage(ctx, opt)
}

// Prune deletes reclaimable build cache
func (w *Worker) Prune(ctx context.Context, ch chan client.UsageInfo) error {
	return w.CacheManager.Prune(ctx, ch)
}

// Exporter returns exporter by name
func (w *Worker) Exporter(name string) (exporter.Exporter, error) {
	exp, ok := w.Exporters[name]
	if !ok {
		return nil, errors.Errorf("exporter %q could not be found", name)
	}
	return exp, nil
}

// GetRemote returns a remote snapshot reference for a local one
func (w *Worker) GetRemote(ctx context.Context, ref cache.ImmutableRef, createIfNeeded bool) (*solver.Remote, error) {
	return nil, errors.Errorf("getremote not implemented")
}

// FromRemote converts a remote snapshot reference to a local one
func (w *Worker) FromRemote(ctx context.Context, remote *solver.Remote) (cache.ImmutableRef, error) {
	rootfs, err := getLayers(ctx, remote.Descriptors)
	if err != nil {
		return nil, err
	}

	layers := make([]xfer.DownloadDescriptor, 0, len(rootfs))

	for _, l := range rootfs {
		// ongoing.add(desc)
		layers = append(layers, &layerDescriptor{
			desc:     l.Blob,
			diffID:   layer.DiffID(l.Diff.Digest),
			provider: remote.Provider,
			w:        w,
			pctx:     ctx,
		})
	}

	defer func() {
		for _, l := range rootfs {
			w.ContentStore.Delete(context.TODO(), l.Blob.Digest)
		}
	}()

	r := image.NewRootFS()
	rootFS, release, err := w.DownloadManager.Download(ctx, *r, runtime.GOOS, layers, &discardProgress{})
	if err != nil {
		return nil, err
	}
	defer release()

	ref, err := w.CacheManager.GetFromSnapshotter(ctx, string(rootFS.ChainID()), cache.WithDescription(fmt.Sprintf("imported %s", remote.Descriptors[len(remote.Descriptors)-1].Digest)))
	if err != nil {
		return nil, err
	}
	return ref, nil
}

type discardProgress struct{}

func (*discardProgress) WriteProgress(_ pkgprogress.Progress) error {
	return nil
}

// Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error)
type layerDescriptor struct {
	provider content.Provider
	desc     ocispec.Descriptor
	diffID   layer.DiffID
	// ref      ctdreference.Spec
	w    *Worker
	pctx context.Context
}

func (ld *layerDescriptor) Key() string {
	return "v2:" + ld.desc.Digest.String()
}

func (ld *layerDescriptor) ID() string {
	return ld.desc.Digest.String()
}

func (ld *layerDescriptor) DiffID() (layer.DiffID, error) {
	return ld.diffID, nil
}

func (ld *layerDescriptor) Download(ctx context.Context, progressOutput pkgprogress.Output) (io.ReadCloser, int64, error) {
	done := oneOffProgress(ld.pctx, fmt.Sprintf("pulling %s", ld.desc.Digest))
	if err := contentutil.Copy(ctx, ld.w.ContentStore, ld.provider, ld.desc); err != nil {
		return nil, 0, done(err)
	}
	done(nil)

	ra, err := ld.w.ContentStore.ReaderAt(ctx, ld.desc)
	if err != nil {
		return nil, 0, err
	}

	return ioutil.NopCloser(content.NewReader(ra)), ld.desc.Size, nil
}

func (ld *layerDescriptor) Close() {
	// ld.is.ContentStore.Delete(context.TODO(), ld.desc.Digest)
}

func (ld *layerDescriptor) Registered(diffID layer.DiffID) {
	// Cache mapping from this layer's DiffID to the blobsum
	ld.w.V2MetadataService.Add(diffID, distmetadata.V2Metadata{Digest: ld.desc.Digest})
}

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

type resolveImageConfig interface {
	ResolveImageConfig(ctx context.Context, ref string, platform *ocispec.Platform) (digest.Digest, []byte, error)
}
