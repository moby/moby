package containerimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	ctdreference "github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/schema1"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	pkgprogress "github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/tracing"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	netcontext "golang.org/x/net/context"
)

type SourceOpt struct {
	SessionManager  *session.Manager
	ContentStore    content.Store
	CacheAccessor   cache.Accessor
	ReferenceStore  reference.Store
	DownloadManager distribution.RootFSDownloadManager
	MetadataStore   metadata.V2MetadataService
}

type imageSource struct {
	SourceOpt
	g flightcontrol.Group
}

func NewSource(opt SourceOpt) (source.Source, error) {
	is := &imageSource{
		SourceOpt: opt,
	}

	return is, nil
}

func (is *imageSource) ID() string {
	return source.DockerImageScheme
}

func (is *imageSource) getResolver(ctx context.Context) remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Client:      tracing.DefaultClient,
		Credentials: is.getCredentialsFromSession(ctx),
	})
}

func (is *imageSource) getCredentialsFromSession(ctx context.Context) func(string) (string, string, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil
	}
	return func(host string) (string, string, error) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		caller, err := is.SessionManager.Get(timeoutCtx, id)
		if err != nil {
			return "", "", err
		}

		return auth.CredentialsFunc(tracing.ContextWithSpanFromContext(context.TODO(), ctx), caller)(host)
	}
}

func (is *imageSource) ResolveImageConfig(ctx context.Context, ref string) (digest.Digest, []byte, error) {
	// type t struct {
	// 	dgst digest.Digest
	// 	dt   []byte
	// }
	// res, err := is.g.Do(ctx, ref, func(ctx context.Context) (interface{}, error) {
	// 	dgst, dt, err := imageutil.Config(ctx, ref, is.getResolver(ctx), is.ContentStore)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	return &t{dgst: dgst, dt: dt}, nil
	// })
	// if err != nil {
	// 	return "", nil, err
	// }
	// typed := res.(*t)
	// return typed.dgst, typed.dt, nil
	return "", nil, errors.Errorf("not-implemented")
}

func (is *imageSource) Resolve(ctx context.Context, id source.Identifier) (source.SourceInstance, error) {
	imageIdentifier, ok := id.(*source.ImageIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid image identifier %v", id)
	}

	p := &puller{
		src:      imageIdentifier,
		is:       is,
		resolver: is.getResolver(ctx),
	}
	return p, nil
}

type puller struct {
	is          *imageSource
	resolveOnce sync.Once
	src         *source.ImageIdentifier
	desc        ocispec.Descriptor
	ref         string
	resolveErr  error
	resolver    remotes.Resolver
}

func (p *puller) resolve(ctx context.Context) error {
	p.resolveOnce.Do(func() {
		resolveProgressDone := oneOffProgress(ctx, "resolve "+p.src.Reference.String())

		dgst := p.src.Reference.Digest()
		if dgst != "" {
			info, err := p.is.ContentStore.Info(ctx, dgst)
			if err == nil {
				p.ref = p.src.Reference.String()
				ra, err := p.is.ContentStore.ReaderAt(ctx, dgst)
				if err == nil {
					mt, err := imageutil.DetectManifestMediaType(ra)
					if err == nil {
						p.desc = ocispec.Descriptor{
							Size:      info.Size,
							Digest:    dgst,
							MediaType: mt,
						}
						resolveProgressDone(nil)
						return
					}
				}
			}
		}

		ref, desc, err := p.resolver.Resolve(ctx, p.src.Reference.String())
		if err != nil {
			p.resolveErr = err
			resolveProgressDone(err)
			return
		}
		p.desc = desc
		p.ref = ref
		resolveProgressDone(nil)
	})
	return p.resolveErr
}

func (p *puller) CacheKey(ctx context.Context) (string, error) {
	if err := p.resolve(ctx); err != nil {
		return "", err
	}
	return p.desc.Digest.String(), nil
}

func (p *puller) Snapshot(ctx context.Context) (cache.ImmutableRef, error) {
	if err := p.resolve(ctx); err != nil {
		return nil, err
	}

	ongoing := newJobs(p.ref)

	pctx, stopProgress := context.WithCancel(ctx)

	go showProgress(pctx, ongoing, p.is.ContentStore)

	fetcher, err := p.resolver.Fetcher(ctx, p.ref)
	if err != nil {
		stopProgress()
		return nil, err
	}

	// TODO: need a wrapper snapshot interface that combines content
	// and snapshots as 1) buildkit shouldn't have a dependency on contentstore
	// or 2) cachemanager should manage the contentstore
	handlers := []images.Handler{
		images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			switch desc.MediaType {
			case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest,
				images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex,
				images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
			default:
				return nil, images.ErrSkipDesc
			}
			ongoing.add(desc)
			return nil, nil
		}),
	}
	var schema1Converter *schema1.Converter
	if p.desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		schema1Converter = schema1.NewConverter(p.is.ContentStore, fetcher)
		handlers = append(handlers, schema1Converter)
	} else {
		handlers = append(handlers,
			remotes.FetchHandler(p.is.ContentStore, fetcher),
			images.ChildrenHandler(p.is.ContentStore, platforms.Default()),
		)
	}

	if err := images.Dispatch(ctx, images.Handlers(handlers...), p.desc); err != nil {
		stopProgress()
		return nil, err
	}
	stopProgress()

	mfst, err := images.Manifest(ctx, p.is.ContentStore, p.desc, platforms.Default())
	if err != nil {
		return nil, err
	}

	config, err := images.Config(ctx, p.is.ContentStore, p.desc, platforms.Default())
	if err != nil {
		return nil, err
	}

	dt, err := content.ReadBlob(ctx, p.is.ContentStore, config.Digest)
	if err != nil {
		return nil, err
	}

	var img ocispec.Image
	if err := json.Unmarshal(dt, &img); err != nil {
		return nil, err
	}

	if len(mfst.Layers) != len(img.RootFS.DiffIDs) {
		return nil, errors.Errorf("invalid config for manifest")
	}

	pchan := make(chan pkgprogress.Progress, 10)

	go func() {
		for p := range pchan {
			logrus.Debugf("progress %+v", p)
		}
	}()

	layers := make([]xfer.DownloadDescriptor, 0, len(mfst.Layers))

	for i, desc := range mfst.Layers {
		layers = append(layers, &layerDescriptor{
			desc:    desc,
			diffID:  layer.DiffID(img.RootFS.DiffIDs[i]),
			fetcher: fetcher,
			ref:     p.src.Reference,
			is:      p.is,
		})
	}

	r := image.NewRootFS()
	rootFS, release, err := p.is.DownloadManager.Download(ctx, *r, runtime.GOOS, layers, pkgprogress.ChanOutput(pchan))
	if err != nil {
		return nil, err
	}

	ref, err := p.is.CacheAccessor.Get(ctx, string(rootFS.ChainID()), cache.WithDescription(fmt.Sprintf("pulled from %s", p.ref)))
	release()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error)
type layerDescriptor struct {
	is      *imageSource
	fetcher remotes.Fetcher
	desc    ocispec.Descriptor
	diffID  layer.DiffID
	ref     ctdreference.Spec
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

func (ld *layerDescriptor) Download(ctx netcontext.Context, progressOutput pkgprogress.Output) (io.ReadCloser, int64, error) {
	rc, err := ld.fetcher.Fetch(ctx, ld.desc)
	if err != nil {
		return nil, 0, err
	}
	defer rc.Close()

	// TODO: progress
	if err := content.WriteBlob(ctx, ld.is.ContentStore, ld.desc.Digest.String(), rc, ld.desc.Size, ld.desc.Digest); err != nil {
		return nil, 0, err
	}

	ra, err := ld.is.ContentStore.ReaderAt(ctx, ld.desc.Digest)
	if err != nil {
		return nil, 0, err
	}

	return ioutils.NewReadCloserWrapper(content.NewReader(ra), func() error {
		return ld.is.ContentStore.Delete(context.TODO(), ld.desc.Digest)
	}), ld.desc.Size, nil
}

func (ld *layerDescriptor) Close() {
	ld.is.ContentStore.Delete(context.TODO(), ld.desc.Digest)
}

func (ld *layerDescriptor) Registered(diffID layer.DiffID) {
	// Cache mapping from this layer's DiffID to the blobsum
	ld.is.MetadataStore.Add(diffID, metadata.V2Metadata{Digest: ld.desc.Digest, SourceRepository: ld.ref.Locator})
}

func showProgress(ctx context.Context, ongoing *jobs, cs content.Store) {
	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		statuses = map[string]statusInfo{}
		done     bool
	)
	defer ticker.Stop()

	pw, _, ctx := progress.FromContext(ctx)
	defer pw.Close()

	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			done = true
		}

		resolved := "resolved"
		if !ongoing.isResolved() {
			resolved = "resolving"
		}
		statuses[ongoing.name] = statusInfo{
			Ref:    ongoing.name,
			Status: resolved,
		}

		actives := make(map[string]statusInfo)

		if !done {
			active, err := cs.ListStatuses(ctx, "")
			if err != nil {
				// log.G(ctx).WithError(err).Error("active check failed")
				continue
			}
			// update status of active entries!
			for _, active := range active {
				actives[active.Ref] = statusInfo{
					Ref:       active.Ref,
					Status:    "downloading",
					Offset:    active.Offset,
					Total:     active.Total,
					StartedAt: active.StartedAt,
					UpdatedAt: active.UpdatedAt,
				}
			}
		}

		// now, update the items in jobs that are not in active
		for _, j := range ongoing.jobs() {
			refKey := remotes.MakeRefKey(ctx, j.Descriptor)
			if a, ok := actives[refKey]; ok {
				started := j.started
				pw.Write(j.Digest.String(), progress.Status{
					Action:  a.Status,
					Total:   int(a.Total),
					Current: int(a.Offset),
					Started: &started,
				})
				continue
			}

			if !j.done {
				info, err := cs.Info(context.TODO(), j.Digest)
				if err != nil {
					if errdefs.IsNotFound(err) {
						pw.Write(j.Digest.String(), progress.Status{
							Action: "waiting",
						})
						continue
					}
				} else {
					j.done = true
				}

				if done || j.done {
					started := j.started
					createdAt := info.CreatedAt
					pw.Write(j.Digest.String(), progress.Status{
						Action:    "done",
						Current:   int(info.Size),
						Total:     int(info.Size),
						Completed: &createdAt,
						Started:   &started,
					})
				}
			}
		}
		if done {
			return
		}
	}
}

// jobs provides a way of identifying the download keys for a particular task
// encountering during the pull walk.
//
// This is very minimal and will probably be replaced with something more
// featured.
type jobs struct {
	name     string
	added    map[digest.Digest]job
	mu       sync.Mutex
	resolved bool
}

type job struct {
	ocispec.Descriptor
	done    bool
	started time.Time
}

func newJobs(name string) *jobs {
	return &jobs{
		name:  name,
		added: make(map[digest.Digest]job),
	}
}

func (j *jobs) add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.added[desc.Digest] = job{
		Descriptor: desc,
		started:    time.Now(),
	}
}

func (j *jobs) jobs() []job {
	j.mu.Lock()
	defer j.mu.Unlock()

	descs := make([]job, 0, len(j.added))
	for _, j := range j.added {
		descs = append(descs, j)
	}
	return descs
}

func (j *jobs) isResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

type statusInfo struct {
	Ref       string
	Status    string
	Offset    int64
	Total     int64
	StartedAt time.Time
	UpdatedAt time.Time
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
