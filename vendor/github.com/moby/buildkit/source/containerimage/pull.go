package containerimage

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	containerderrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/estargz"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/util/progress/controller"
	"github.com/moby/buildkit/util/pull"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TODO: break apart containerd specifics like contentstore so the resolver
// code can be used with any implementation

type SourceOpt struct {
	Snapshotter   snapshot.Snapshotter
	ContentStore  content.Store
	Applier       diff.Applier
	CacheAccessor cache.Accessor
	ImageStore    images.Store // optional
	RegistryHosts docker.RegistryHosts
	LeaseManager  leases.Manager
}

type Source struct {
	SourceOpt
	g flightcontrol.Group
}

var _ source.Source = &Source{}

func NewSource(opt SourceOpt) (*Source, error) {
	is := &Source{
		SourceOpt: opt,
	}

	return is, nil
}

func (is *Source) ID() string {
	return srctypes.DockerImageScheme
}

func (is *Source) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	type t struct {
		dgst digest.Digest
		dt   []byte
	}
	key := ref
	if platform := opt.Platform; platform != nil {
		key += platforms.Format(*platform)
	}

	rm, err := source.ParseImageResolveMode(opt.ResolveMode)
	if err != nil {
		return "", nil, err
	}
	key += rm.String()

	res, err := is.g.Do(ctx, key, func(ctx context.Context) (interface{}, error) {
		res := resolver.DefaultPool.GetResolver(is.RegistryHosts, ref, "pull", sm, g).WithImageStore(is.ImageStore, rm)
		dgst, dt, err := imageutil.Config(ctx, ref, res, is.ContentStore, is.LeaseManager, opt.Platform)
		if err != nil {
			return nil, err
		}
		return &t{dgst: dgst, dt: dt}, nil
	})
	if err != nil {
		return "", nil, err
	}
	typed := res.(*t)
	return typed.dgst, typed.dt, nil
}

func (is *Source) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, vtx solver.Vertex) (source.SourceInstance, error) {
	imageIdentifier, ok := id.(*source.ImageIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid image identifier %v", id)
	}

	platform := platforms.DefaultSpec()
	if imageIdentifier.Platform != nil {
		platform = *imageIdentifier.Platform
	}

	pullerUtil := &pull.Puller{
		ContentStore: is.ContentStore,
		Platform:     platform,
		Src:          imageIdentifier.Reference,
	}
	p := &puller{
		CacheAccessor:  is.CacheAccessor,
		LeaseManager:   is.LeaseManager,
		Puller:         pullerUtil,
		id:             imageIdentifier,
		RegistryHosts:  is.RegistryHosts,
		ImageStore:     is.ImageStore,
		Mode:           imageIdentifier.ResolveMode,
		Ref:            imageIdentifier.Reference.String(),
		SessionManager: sm,
		vtx:            vtx,
	}
	return p, nil
}

type puller struct {
	CacheAccessor  cache.Accessor
	LeaseManager   leases.Manager
	RegistryHosts  docker.RegistryHosts
	ImageStore     images.Store
	Mode           source.ResolveMode
	Ref            string
	SessionManager *session.Manager
	id             *source.ImageIdentifier
	vtx            solver.Vertex

	g                flightcontrol.Group
	cacheKeyErr      error
	cacheKeyDone     bool
	releaseTmpLeases func(context.Context) error
	descHandlers     cache.DescHandlers
	manifest         *pull.PulledManifests
	manifestKey      string
	configKey        string
	*pull.Puller
}

func mainManifestKey(ctx context.Context, desc ocispecs.Descriptor, platform ocispecs.Platform) (digest.Digest, error) {
	dt, err := json.Marshal(struct {
		Digest  digest.Digest
		OS      string
		Arch    string
		Variant string `json:",omitempty"`
	}{
		Digest:  desc.Digest,
		OS:      platform.OS,
		Arch:    platform.Architecture,
		Variant: platform.Variant,
	})
	if err != nil {
		return "", err
	}
	return digest.FromBytes(dt), nil
}

func (p *puller) CacheKey(ctx context.Context, g session.Group, index int) (cacheKey string, imgDigest string, cacheOpts solver.CacheOpts, cacheDone bool, err error) {
	p.Puller.Resolver = resolver.DefaultPool.GetResolver(p.RegistryHosts, p.Ref, "pull", p.SessionManager, g).WithImageStore(p.ImageStore, p.id.ResolveMode)

	// progressFactory needs the outer context, the context in `p.g.Do` will
	// be canceled before the progress output is complete
	progressFactory := progress.FromContext(ctx)

	_, err = p.g.Do(ctx, "", func(ctx context.Context) (_ interface{}, err error) {
		if p.cacheKeyErr != nil || p.cacheKeyDone == true {
			return nil, p.cacheKeyErr
		}
		defer func() {
			if !errdefs.IsCanceled(ctx, err) {
				p.cacheKeyErr = err
			}
		}()
		ctx, done, err := leaseutil.WithLease(ctx, p.LeaseManager, leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
		if err != nil {
			return nil, err
		}
		p.releaseTmpLeases = done
		defer imageutil.AddLease(done)

		resolveProgressDone := oneOffProgress(ctx, "resolve "+p.Src.String())
		defer func() {
			resolveProgressDone(err)
		}()

		p.manifest, err = p.PullManifests(ctx)
		if err != nil {
			return nil, err
		}

		if len(p.manifest.Descriptors) > 0 {
			progressController := &controller.Controller{
				WriterFactory: progressFactory,
			}
			if p.vtx != nil {
				progressController.Digest = p.vtx.Digest()
				progressController.Name = p.vtx.Name()
				progressController.ProgressGroup = p.vtx.Options().ProgressGroup
			}

			p.descHandlers = cache.DescHandlers(make(map[digest.Digest]*cache.DescHandler))
			for i, desc := range p.manifest.Descriptors {
				labels := snapshots.FilterInheritedLabels(desc.Annotations)
				if labels == nil {
					labels = make(map[string]string)
				}
				for k, v := range estargz.SnapshotLabels(p.manifest.Ref, p.manifest.Descriptors, i) {
					labels[k] = v
				}
				p.descHandlers[desc.Digest] = &cache.DescHandler{
					Provider:       p.manifest.Provider,
					Progress:       progressController,
					SnapshotLabels: labels,
					Annotations:    desc.Annotations,
					Ref:            p.manifest.Ref,
				}
			}
		}

		desc := p.manifest.MainManifestDesc
		k, err := mainManifestKey(ctx, desc, p.Platform)
		if err != nil {
			return nil, err
		}
		p.manifestKey = k.String()

		dt, err := content.ReadBlob(ctx, p.ContentStore, p.manifest.ConfigDesc)
		if err != nil {
			return nil, err
		}
		p.configKey = cacheKeyFromConfig(dt).String()
		p.cacheKeyDone = true
		return nil, nil
	})
	if err != nil {
		return "", "", nil, false, err
	}

	cacheOpts = solver.CacheOpts(make(map[interface{}]interface{}))
	for dgst, descHandler := range p.descHandlers {
		cacheOpts[cache.DescHandlerKey(dgst)] = descHandler
	}

	cacheDone = index > 0
	if index == 0 || p.configKey == "" {
		return p.manifestKey, p.manifest.MainManifestDesc.Digest.String(), cacheOpts, cacheDone, nil
	}
	return p.configKey, p.manifest.MainManifestDesc.Digest.String(), cacheOpts, cacheDone, nil
}

func (p *puller) Snapshot(ctx context.Context, g session.Group) (ir cache.ImmutableRef, err error) {
	p.Puller.Resolver = resolver.DefaultPool.GetResolver(p.RegistryHosts, p.Ref, "pull", p.SessionManager, g).WithImageStore(p.ImageStore, p.id.ResolveMode)

	if len(p.manifest.Descriptors) == 0 {
		return nil, nil
	}
	defer func() {
		if p.releaseTmpLeases != nil {
			p.releaseTmpLeases(context.TODO())
		}
	}()

	var current cache.ImmutableRef
	defer func() {
		if err != nil && current != nil {
			current.Release(context.TODO())
		}
	}()

	var parent cache.ImmutableRef
	setWindowsLayerType := p.Platform.OS == "windows" && runtime.GOOS != "windows"
	for _, layerDesc := range p.manifest.Descriptors {
		parent = current
		current, err = p.CacheAccessor.GetByBlob(ctx, layerDesc, parent,
			p.descHandlers, cache.WithImageRef(p.manifest.Ref))
		if parent != nil {
			parent.Release(context.TODO())
		}
		if err != nil {
			return nil, err
		}
		if setWindowsLayerType {
			if err := current.SetLayerType("windows"); err != nil {
				return nil, err
			}
		}
	}

	for _, desc := range p.manifest.Nonlayers {
		if _, err := p.ContentStore.Info(ctx, desc.Digest); containerderrdefs.IsNotFound(err) {
			// manifest or config must have gotten gc'd after CacheKey, re-pull them
			ctx, done, err := leaseutil.WithLease(ctx, p.LeaseManager, leaseutil.MakeTemporary)
			if err != nil {
				return nil, err
			}
			defer done(ctx)

			if _, err := p.PullManifests(ctx); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		if err := p.LeaseManager.AddResource(ctx, leases.Lease{ID: current.ID()}, leases.Resource{
			ID:   desc.Digest.String(),
			Type: "content",
		}); err != nil {
			return nil, err
		}
	}

	if p.id.RecordType != "" && current.GetRecordType() == "" {
		if err := current.SetRecordType(p.id.RecordType); err != nil {
			return nil, err
		}
	}

	return current, nil
}

// cacheKeyFromConfig returns a stable digest from image config. If image config
// is a known oci image we will use chainID of layers.
func cacheKeyFromConfig(dt []byte) digest.Digest {
	var img ocispecs.Image
	err := json.Unmarshal(dt, &img)
	if err != nil {
		return digest.FromBytes(dt)
	}
	if img.RootFS.Type != "layers" || len(img.RootFS.DiffIDs) == 0 {
		return ""
	}
	return identity.ChainID(img.RootFS.DiffIDs)
}

func oneOffProgress(ctx context.Context, id string) func(err error) error {
	pw, _, _ := progress.NewFromContext(ctx)
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
