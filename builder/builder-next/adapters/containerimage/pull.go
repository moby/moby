package containerimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/gc"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	ctdreference "github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/schema1"
	distreference "github.com/docker/distribution/reference"
	dimages "github.com/docker/docker/daemon/images"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	pkgprogress "github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/resolver"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// SourceOpt is options for creating the image source
type SourceOpt struct {
	ContentStore    content.Store
	CacheAccessor   cache.Accessor
	ReferenceStore  reference.Store
	DownloadManager *xfer.LayerDownloadManager
	MetadataStore   metadata.V2MetadataService
	ImageStore      image.Store
	RegistryHosts   docker.RegistryHosts
	LayerStore      layer.Store
	LeaseManager    leases.Manager
	GarbageCollect  func(ctx context.Context) (gc.Stats, error)
}

// Source is the source implementation for accessing container images
type Source struct {
	SourceOpt
	g flightcontrol.Group
}

// NewSource creates a new image source
func NewSource(opt SourceOpt) (*Source, error) {
	return &Source{SourceOpt: opt}, nil
}

// ID returns image scheme identifier
func (is *Source) ID() string {
	return srctypes.DockerImageScheme
}

func (is *Source) resolveLocal(refStr string) (*image.Image, error) {
	ref, err := distreference.ParseNormalizedNamed(refStr)
	if err != nil {
		return nil, err
	}
	dgst, err := is.ReferenceStore.Get(ref)
	if err != nil {
		return nil, err
	}
	img, err := is.ImageStore.Get(image.ID(dgst))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (is *Source) resolveRemote(ctx context.Context, ref string, platform *ocispec.Platform, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	type t struct {
		dgst digest.Digest
		dt   []byte
	}
	p := platforms.DefaultSpec()
	if platform != nil {
		p = *platform
	}
	// key is used to synchronize resolutions that can happen in parallel when doing multi-stage.
	key := "getconfig::" + ref + "::" + platforms.Format(p)
	res, err := is.g.Do(ctx, key, func(ctx context.Context) (interface{}, error) {
		res := resolver.DefaultPool.GetResolver(is.RegistryHosts, ref, "pull", sm, g)
		dgst, dt, err := imageutil.Config(ctx, ref, res, is.ContentStore, is.LeaseManager, platform)
		if err != nil {
			return nil, err
		}
		return &t{dgst: dgst, dt: dt}, nil
	})
	var typed *t
	if err != nil {
		return "", nil, err
	}
	typed = res.(*t)
	return typed.dgst, typed.dt, nil
}

// ResolveImageConfig returns image config for an image
func (is *Source) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	resolveMode, err := source.ParseImageResolveMode(opt.ResolveMode)
	if err != nil {
		return "", nil, err
	}
	switch resolveMode {
	case source.ResolveModeForcePull:
		dgst, dt, err := is.resolveRemote(ctx, ref, opt.Platform, sm, g)
		// TODO: pull should fallback to local in case of failure to allow offline behavior
		// the fallback doesn't work currently
		return dgst, dt, err
		/*
			if err == nil {
				return dgst, dt, err
			}
			// fallback to local
			dt, err = is.resolveLocal(ref)
			return "", dt, err
		*/

	case source.ResolveModeDefault:
		// default == prefer local, but in the future could be smarter
		fallthrough
	case source.ResolveModePreferLocal:
		img, err := is.resolveLocal(ref)
		if err == nil {
			if opt.Platform != nil && !platformMatches(img, opt.Platform) {
				logrus.WithField("ref", ref).Debugf("Requested build platform %s does not match local image platform %s, checking remote",
					path.Join(opt.Platform.OS, opt.Platform.Architecture, opt.Platform.Variant),
					path.Join(img.OS, img.Architecture, img.Variant),
				)
			} else {
				return "", img.RawJSON(), err
			}
		}
		// fallback to remote
		return is.resolveRemote(ctx, ref, opt.Platform, sm, g)
	}
	// should never happen
	return "", nil, fmt.Errorf("builder cannot resolve image %s: invalid mode %q", ref, opt.ResolveMode)
}

// Resolve returns access to pulling for an identifier
func (is *Source) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, vtx solver.Vertex) (source.SourceInstance, error) {
	imageIdentifier, ok := id.(*source.ImageIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid image identifier %v", id)
	}

	platform := platforms.DefaultSpec()
	if imageIdentifier.Platform != nil {
		platform = *imageIdentifier.Platform
	}

	p := &puller{
		src: imageIdentifier,
		is:  is,
		// resolver: is.getResolver(is.RegistryHosts, imageIdentifier.Reference.String(), sm, g),
		platform: platform,
		sm:       sm,
	}
	return p, nil
}

type puller struct {
	is               *Source
	resolveLocalOnce sync.Once
	g                flightcontrol.Group
	src              *source.ImageIdentifier
	desc             ocispec.Descriptor
	ref              string
	config           []byte
	platform         ocispec.Platform
	sm               *session.Manager
}

func (p *puller) resolver(g session.Group) remotes.Resolver {
	return resolver.DefaultPool.GetResolver(p.is.RegistryHosts, p.src.Reference.String(), "pull", p.sm, g)
}

func (p *puller) mainManifestKey(platform ocispec.Platform) (digest.Digest, error) {
	dt, err := json.Marshal(struct {
		Digest  digest.Digest
		OS      string
		Arch    string
		Variant string `json:",omitempty"`
	}{
		Digest:  p.desc.Digest,
		OS:      platform.OS,
		Arch:    platform.Architecture,
		Variant: platform.Variant,
	})
	if err != nil {
		return "", err
	}
	return digest.FromBytes(dt), nil
}

func (p *puller) resolveLocal() {
	p.resolveLocalOnce.Do(func() {
		dgst := p.src.Reference.Digest()
		if dgst != "" {
			info, err := p.is.ContentStore.Info(context.TODO(), dgst)
			if err == nil {
				p.ref = p.src.Reference.String()
				desc := ocispec.Descriptor{
					Size:   info.Size,
					Digest: dgst,
				}
				ra, err := p.is.ContentStore.ReaderAt(context.TODO(), desc)
				if err == nil {
					mt, err := imageutil.DetectManifestMediaType(ra)
					if err == nil {
						desc.MediaType = mt
						p.desc = desc
					}
				}
			}
		}

		if p.src.ResolveMode == source.ResolveModeDefault || p.src.ResolveMode == source.ResolveModePreferLocal {
			ref := p.src.Reference.String()
			img, err := p.is.resolveLocal(ref)
			if err == nil {
				if !platformMatches(img, &p.platform) {
					logrus.WithField("ref", ref).Debugf("Requested build platform %s does not match local image platform %s, not resolving",
						path.Join(p.platform.OS, p.platform.Architecture, p.platform.Variant),
						path.Join(img.OS, img.Architecture, img.Variant),
					)
				} else {
					p.config = img.RawJSON()
				}
			}
		}
	})
}

func (p *puller) resolve(ctx context.Context, g session.Group) error {
	_, err := p.g.Do(ctx, "", func(ctx context.Context) (_ interface{}, err error) {
		resolveProgressDone := oneOffProgress(ctx, "resolve "+p.src.Reference.String())
		defer func() {
			resolveProgressDone(err)
		}()

		ref, err := distreference.ParseNormalizedNamed(p.src.Reference.String())
		if err != nil {
			return nil, err
		}

		if p.desc.Digest == "" && p.config == nil {
			origRef, desc, err := p.resolver(g).Resolve(ctx, ref.String())
			if err != nil {
				return nil, err
			}

			p.desc = desc
			p.ref = origRef
		}

		// Schema 1 manifests cannot be resolved to an image config
		// since the conversion must take place after all the content
		// has been read.
		// It may be possible to have a mapping between schema 1 manifests
		// and the schema 2 manifests they are converted to.
		if p.config == nil && p.desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ref, err := distreference.WithDigest(ref, p.desc.Digest)
			if err != nil {
				return nil, err
			}
			_, dt, err := p.is.ResolveImageConfig(ctx, ref.String(), llb.ResolveImageConfigOpt{Platform: &p.platform, ResolveMode: resolveModeToString(p.src.ResolveMode)}, p.sm, g)
			if err != nil {
				return nil, err
			}

			p.config = dt
		}
		return nil, nil
	})
	return err
}

func (p *puller) CacheKey(ctx context.Context, g session.Group, index int) (string, string, solver.CacheOpts, bool, error) {
	p.resolveLocal()

	if p.desc.Digest != "" && index == 0 {
		dgst, err := p.mainManifestKey(p.platform)
		if err != nil {
			return "", "", nil, false, err
		}
		return dgst.String(), p.desc.Digest.String(), nil, false, nil
	}

	if p.config != nil {
		k := cacheKeyFromConfig(p.config).String()
		if k == "" {
			return digest.FromBytes(p.config).String(), digest.FromBytes(p.config).String(), nil, true, nil
		}
		return k, k, nil, true, nil
	}

	if err := p.resolve(ctx, g); err != nil {
		return "", "", nil, false, err
	}

	if p.desc.Digest != "" && index == 0 {
		dgst, err := p.mainManifestKey(p.platform)
		if err != nil {
			return "", "", nil, false, err
		}
		return dgst.String(), p.desc.Digest.String(), nil, false, nil
	}

	if len(p.config) == 0 && p.desc.MediaType != images.MediaTypeDockerSchema1Manifest {
		return "", "", nil, false, errors.Errorf("invalid empty config file resolved for %s", p.src.Reference.String())
	}

	k := cacheKeyFromConfig(p.config).String()
	if k == "" || p.desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		dgst, err := p.mainManifestKey(p.platform)
		if err != nil {
			return "", "", nil, false, err
		}
		return dgst.String(), p.desc.Digest.String(), nil, true, nil
	}

	return k, k, nil, true, nil
}

func (p *puller) getRef(ctx context.Context, diffIDs []layer.DiffID, opts ...cache.RefOption) (cache.ImmutableRef, error) {
	var parent cache.ImmutableRef
	if len(diffIDs) > 1 {
		var err error
		parent, err = p.getRef(ctx, diffIDs[:len(diffIDs)-1], opts...)
		if err != nil {
			return nil, err
		}
		defer parent.Release(context.TODO())
	}
	return p.is.CacheAccessor.GetByBlob(ctx, ocispec.Descriptor{
		Annotations: map[string]string{
			"containerd.io/uncompressed": diffIDs[len(diffIDs)-1].String(),
		},
	}, parent, opts...)
}

func (p *puller) Snapshot(ctx context.Context, g session.Group) (cache.ImmutableRef, error) {
	p.resolveLocal()
	if len(p.config) == 0 {
		if err := p.resolve(ctx, g); err != nil {
			return nil, err
		}
	}

	if p.config != nil {
		img, err := p.is.ImageStore.Get(image.ID(digest.FromBytes(p.config)))
		if err == nil {
			if len(img.RootFS.DiffIDs) == 0 {
				return nil, nil
			}
			l, err := p.is.LayerStore.Get(img.RootFS.ChainID())
			if err == nil {
				layer.ReleaseAndLog(p.is.LayerStore, l)
				ref, err := p.getRef(ctx, img.RootFS.DiffIDs, cache.WithDescription(fmt.Sprintf("from local %s", p.ref)))
				if err != nil {
					return nil, err
				}
				return ref, nil
			}
		}
	}

	ongoing := newJobs(p.ref)

	ctx, done, err := leaseutil.WithLease(ctx, p.is.LeaseManager, leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}
	defer func() {
		done(context.TODO())
		if p.is.GarbageCollect != nil {
			go p.is.GarbageCollect(context.TODO())
		}
	}()

	pctx, stopProgress := context.WithCancel(ctx)

	pw, _, ctx := progress.NewFromContext(ctx)
	defer pw.Close()

	progressDone := make(chan struct{})
	go func() {
		showProgress(pctx, ongoing, p.is.ContentStore, pw)
		close(progressDone)
	}()
	defer func() {
		<-progressDone
	}()

	fetcher, err := p.resolver(g).Fetcher(ctx, p.ref)
	if err != nil {
		stopProgress()
		return nil, err
	}

	platform := platforms.Only(p.platform)

	var nonLayers []digest.Digest

	var (
		schema1Converter *schema1.Converter
		handlers         []images.Handler
	)
	if p.desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		schema1Converter = schema1.NewConverter(p.is.ContentStore, fetcher)
		handlers = append(handlers, schema1Converter)

		// TODO: Optimize to do dispatch and integrate pulling with download manager,
		// leverage existing blob mapping and layer storage
	} else {
		// TODO: need a wrapper snapshot interface that combines content
		// and snapshots as 1) buildkit shouldn't have a dependency on contentstore
		// or 2) cachemanager should manage the contentstore
		handlers = append(handlers, images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			switch desc.MediaType {
			case images.MediaTypeDockerSchema2Manifest, ocispec.MediaTypeImageManifest,
				images.MediaTypeDockerSchema2ManifestList, ocispec.MediaTypeImageIndex,
				images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
				nonLayers = append(nonLayers, desc.Digest)
			default:
				return nil, images.ErrSkipDesc
			}
			ongoing.add(desc)
			return nil, nil
		}))

		// Get all the children for a descriptor
		childrenHandler := images.ChildrenHandler(p.is.ContentStore)
		// Filter the children by the platform
		childrenHandler = images.FilterPlatforms(childrenHandler, platform)
		// Limit manifests pulled to the best match in an index
		childrenHandler = images.LimitManifests(childrenHandler, platform, 1)

		handlers = append(handlers,
			remotes.FetchHandler(p.is.ContentStore, fetcher),
			childrenHandler,
		)
	}

	if err := images.Dispatch(ctx, images.Handlers(handlers...), nil, p.desc); err != nil {
		stopProgress()
		return nil, err
	}
	defer stopProgress()

	if schema1Converter != nil {
		p.desc, err = schema1Converter.Convert(ctx)
		if err != nil {
			return nil, err
		}
	}

	mfst, err := images.Manifest(ctx, p.is.ContentStore, p.desc, platform)
	if err != nil {
		return nil, err
	}

	config, err := images.Config(ctx, p.is.ContentStore, p.desc, platform)
	if err != nil {
		return nil, err
	}

	dt, err := content.ReadBlob(ctx, p.is.ContentStore, config)
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
	defer close(pchan)

	go func() {
		m := map[string]struct {
			st      time.Time
			limiter *rate.Limiter
		}{}
		for p := range pchan {
			if p.Action == "Extracting" {
				st, ok := m[p.ID]
				if !ok {
					st.st = time.Now()
					st.limiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
					m[p.ID] = st
				}
				var end *time.Time
				if p.LastUpdate || st.limiter.Allow() {
					if p.LastUpdate {
						tm := time.Now()
						end = &tm
					}
					_ = pw.Write("extracting "+p.ID, progress.Status{
						Action:    "extract",
						Started:   &st.st,
						Completed: end,
					})
				}
			}
		}
	}()

	if len(mfst.Layers) == 0 {
		return nil, nil
	}

	layers := make([]xfer.DownloadDescriptor, 0, len(mfst.Layers))

	for i, desc := range mfst.Layers {
		if err := desc.Digest.Validate(); err != nil {
			return nil, errors.Wrap(err, "layer digest could not be validated")
		}
		ongoing.add(desc)
		layers = append(layers, &layerDescriptor{
			desc:    desc,
			diffID:  layer.DiffID(img.RootFS.DiffIDs[i]),
			fetcher: fetcher,
			ref:     p.src.Reference,
			is:      p.is,
		})
	}

	defer func() {
		<-progressDone
	}()

	r := image.NewRootFS()
	rootFS, release, err := p.is.DownloadManager.Download(ctx, *r, layers, pkgprogress.ChanOutput(pchan))
	stopProgress()
	if err != nil {
		return nil, err
	}

	ref, err := p.getRef(ctx, rootFS.DiffIDs, cache.WithDescription(fmt.Sprintf("pulled from %s", p.ref)))
	release()
	if err != nil {
		return nil, err
	}

	// keep manifest blobs until ref is alive for cache
	for _, nl := range nonLayers {
		if err := p.is.LeaseManager.AddResource(ctx, leases.Lease{ID: ref.ID()}, leases.Resource{
			ID:   nl.String(),
			Type: "content",
		}); err != nil {
			return nil, err
		}
	}

	// TODO: handle windows layers for cross platform builds

	if p.src.RecordType != "" && ref.GetRecordType() == "" {
		if err := ref.SetRecordType(p.src.RecordType); err != nil {
			ref.Release(context.TODO())
			return nil, err
		}
	}

	return ref, nil
}

// Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error)
type layerDescriptor struct {
	is      *Source
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

func (ld *layerDescriptor) Download(ctx context.Context, progressOutput pkgprogress.Output) (io.ReadCloser, int64, error) {
	rc, err := ld.fetcher.Fetch(ctx, ld.desc)
	if err != nil {
		return nil, 0, err
	}
	defer rc.Close()

	refKey := remotes.MakeRefKey(ctx, ld.desc)

	ld.is.ContentStore.Abort(ctx, refKey)

	if err := content.WriteBlob(ctx, ld.is.ContentStore, refKey, rc, ld.desc); err != nil {
		ld.is.ContentStore.Abort(ctx, refKey)
		return nil, 0, err
	}

	ra, err := ld.is.ContentStore.ReaderAt(ctx, ld.desc)
	if err != nil {
		return nil, 0, err
	}

	return io.NopCloser(content.NewReader(ra)), ld.desc.Size, nil
}

func (ld *layerDescriptor) Close() {
	// ld.is.ContentStore.Delete(context.TODO(), ld.desc.Digest))
}

func (ld *layerDescriptor) Registered(diffID layer.DiffID) {
	// Cache mapping from this layer's DiffID to the blobsum
	ld.is.MetadataStore.Add(diffID, metadata.V2Metadata{Digest: ld.desc.Digest, SourceRepository: ld.ref.Locator})
}

func showProgress(ctx context.Context, ongoing *jobs, cs content.Store, pw progress.Writer) {
	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		statuses = map[string]statusInfo{}
		done     bool
	)
	defer ticker.Stop()

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
			active, err := cs.ListStatuses(ctx)
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
				_ = pw.Write(j.Digest.String(), progress.Status{
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
					if cerrdefs.IsNotFound(err) {
						// _ = pw.Write(j.Digest.String(), progress.Status{
						// 	Action: "waiting",
						// })
						continue
					}
				} else {
					j.done = true
				}

				if done || j.done {
					started := j.started
					createdAt := info.CreatedAt
					_ = pw.Write(j.Digest.String(), progress.Status{
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
	added    map[digest.Digest]*job
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
		added: make(map[digest.Digest]*job),
	}
}

func (j *jobs) add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.added[desc.Digest]; ok {
		return
	}
	j.added[desc.Digest] = &job{
		Descriptor: desc,
		started:    time.Now(),
	}
}

func (j *jobs) jobs() []*job {
	j.mu.Lock()
	defer j.mu.Unlock()

	descs := make([]*job, 0, len(j.added))
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
	pw, _, _ := progress.NewFromContext(ctx)
	now := time.Now()
	st := progress.Status{
		Started: &now,
	}
	_ = pw.Write(id, st)
	return func(err error) error {
		// TODO: set error on status
		now := time.Now()
		st.Completed = &now
		_ = pw.Write(id, st)
		_ = pw.Close()
		return err
	}
}

// cacheKeyFromConfig returns a stable digest from image config. If image config
// is a known oci image we will use chainID of layers.
func cacheKeyFromConfig(dt []byte) digest.Digest {
	var img ocispec.Image
	err := json.Unmarshal(dt, &img)
	if err != nil {
		logrus.WithError(err).Errorf("failed to unmarshal image config for cache key %v", err)
		return digest.FromBytes(dt)
	}
	if img.RootFS.Type != "layers" || len(img.RootFS.DiffIDs) == 0 {
		return ""
	}
	return identity.ChainID(img.RootFS.DiffIDs)
}

// resolveModeToString is the equivalent of github.com/moby/buildkit/solver/llb.ResolveMode.String()
// FIXME: add String method on source.ResolveMode
func resolveModeToString(rm source.ResolveMode) string {
	switch rm {
	case source.ResolveModeDefault:
		return "default"
	case source.ResolveModeForcePull:
		return "pull"
	case source.ResolveModePreferLocal:
		return "local"
	}
	return ""
}

func platformMatches(img *image.Image, p *ocispec.Platform) bool {
	return dimages.OnlyPlatformWithFallback(*p).Match(ocispec.Platform{
		Architecture: img.Architecture,
		OS:           img.OS,
		OSVersion:    img.OSVersion,
		OSFeatures:   img.OSFeatures,
		Variant:      img.Variant,
	})
}
