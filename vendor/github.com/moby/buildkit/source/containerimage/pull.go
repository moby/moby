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
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
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

type ResolverType int

const (
	ResolverTypeRegistry ResolverType = iota
	ResolverTypeOCILayout
)

type SourceOpt struct {
	Snapshotter   snapshot.Snapshotter
	ContentStore  content.Store
	Applier       diff.Applier
	CacheAccessor cache.Accessor
	ImageStore    images.Store // optional
	RegistryHosts docker.RegistryHosts
	ResolverType
	LeaseManager leases.Manager
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
	if is.ResolverType == ResolverTypeOCILayout {
		return srctypes.OCIScheme
	}
	return srctypes.DockerImageScheme
}

func (is *Source) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	type t struct {
		dgst digest.Digest
		dt   []byte
	}
	var typed *t
	key := ref
	if platform := opt.Platform; platform != nil {
		key += platforms.Format(*platform)
	}
	var (
		rm    source.ResolveMode
		rslvr remotes.Resolver
		err   error
	)

	switch is.ResolverType {
	case ResolverTypeRegistry:
		rm, err = source.ParseImageResolveMode(opt.ResolveMode)
		if err != nil {
			return "", nil, err
		}
		rslvr = resolver.DefaultPool.GetResolver(is.RegistryHosts, ref, "pull", sm, g).WithImageStore(is.ImageStore, rm)
	case ResolverTypeOCILayout:
		rm = source.ResolveModeForcePull
		rslvr = getOCILayoutResolver(opt.Store, sm, g)
	}
	key += rm.String()
	res, err := is.g.Do(ctx, key, func(ctx context.Context) (interface{}, error) {
		dgst, dt, err := imageutil.Config(ctx, ref, rslvr, is.ContentStore, is.LeaseManager, opt.Platform)
		if err != nil {
			return nil, err
		}
		return &t{dgst: dgst, dt: dt}, nil
	})
	if err != nil {
		return "", nil, err
	}
	typed = res.(*t)
	return typed.dgst, typed.dt, nil
}

func (is *Source) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, vtx solver.Vertex) (source.SourceInstance, error) {
	var (
		p          *puller
		platform   = platforms.DefaultSpec()
		pullerUtil *pull.Puller
		mode       source.ResolveMode
		recordType client.UsageRecordType
		ref        reference.Spec
		store      llb.ResolveImageConfigOptStore
		layerLimit *int
	)
	switch is.ResolverType {
	case ResolverTypeRegistry:
		imageIdentifier, ok := id.(*source.ImageIdentifier)
		if !ok {
			return nil, errors.Errorf("invalid image identifier %v", id)
		}

		if imageIdentifier.Platform != nil {
			platform = *imageIdentifier.Platform
		}
		mode = imageIdentifier.ResolveMode
		recordType = imageIdentifier.RecordType
		ref = imageIdentifier.Reference
		layerLimit = imageIdentifier.LayerLimit
	case ResolverTypeOCILayout:
		ociIdentifier, ok := id.(*source.OCIIdentifier)
		if !ok {
			return nil, errors.Errorf("invalid OCI layout identifier %v", id)
		}

		if ociIdentifier.Platform != nil {
			platform = *ociIdentifier.Platform
		}
		mode = source.ResolveModeForcePull // with OCI layout, we always just "pull"
		store = llb.ResolveImageConfigOptStore{
			SessionID: ociIdentifier.SessionID,
			StoreID:   ociIdentifier.StoreID,
		}
		ref = ociIdentifier.Reference
		layerLimit = ociIdentifier.LayerLimit
	default:
		return nil, errors.Errorf("unknown resolver type: %v", is.ResolverType)
	}
	pullerUtil = &pull.Puller{
		ContentStore: is.ContentStore,
		Platform:     platform,
		Src:          ref,
	}
	p = &puller{
		CacheAccessor:  is.CacheAccessor,
		LeaseManager:   is.LeaseManager,
		Puller:         pullerUtil,
		RegistryHosts:  is.RegistryHosts,
		ResolverType:   is.ResolverType,
		ImageStore:     is.ImageStore,
		Mode:           mode,
		RecordType:     recordType,
		Ref:            ref.String(),
		SessionManager: sm,
		vtx:            vtx,
		store:          store,
		layerLimit:     layerLimit,
	}
	return p, nil
}

type puller struct {
	CacheAccessor  cache.Accessor
	LeaseManager   leases.Manager
	RegistryHosts  docker.RegistryHosts
	ImageStore     images.Store
	Mode           source.ResolveMode
	RecordType     client.UsageRecordType
	Ref            string
	SessionManager *session.Manager
	layerLimit     *int
	vtx            solver.Vertex
	ResolverType
	store llb.ResolveImageConfigOptStore

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

func mainManifestKey(ctx context.Context, desc ocispecs.Descriptor, platform ocispecs.Platform, layerLimit *int) (digest.Digest, error) {
	dt, err := json.Marshal(struct {
		Digest  digest.Digest
		OS      string
		Arch    string
		Variant string `json:",omitempty"`
		Limit   *int   `json:",omitempty"`
	}{
		Digest:  desc.Digest,
		OS:      platform.OS,
		Arch:    platform.Architecture,
		Variant: platform.Variant,
		Limit:   layerLimit,
	})
	if err != nil {
		return "", err
	}
	return digest.FromBytes(dt), nil
}

func (p *puller) CacheKey(ctx context.Context, g session.Group, index int) (cacheKey string, imgDigest string, cacheOpts solver.CacheOpts, cacheDone bool, err error) {
	var getResolver pull.SessionResolver
	switch p.ResolverType {
	case ResolverTypeRegistry:
		resolver := resolver.DefaultPool.GetResolver(p.RegistryHosts, p.Ref, "pull", p.SessionManager, g).WithImageStore(p.ImageStore, p.Mode)
		p.Puller.Resolver = resolver
		getResolver = func(g session.Group) remotes.Resolver { return resolver.WithSession(g) }
	case ResolverTypeOCILayout:
		resolver := getOCILayoutResolver(p.store, p.SessionManager, g)
		p.Puller.Resolver = resolver
		// OCILayout has no need for session
		getResolver = func(g session.Group) remotes.Resolver { return resolver }
	default:
	}

	// progressFactory needs the outer context, the context in `p.g.Do` will
	// be canceled before the progress output is complete
	progressFactory := progress.FromContext(ctx)

	_, err = p.g.Do(ctx, "", func(ctx context.Context) (_ interface{}, err error) {
		if p.cacheKeyErr != nil || p.cacheKeyDone {
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

		resolveProgressDone := progress.OneOff(ctx, "resolve "+p.Src.String())
		defer func() {
			resolveProgressDone(err)
		}()

		p.manifest, err = p.PullManifests(ctx, getResolver)
		if err != nil {
			return nil, err
		}

		if ll := p.layerLimit; ll != nil {
			if *ll > len(p.manifest.Descriptors) {
				return nil, errors.Errorf("layer limit %d is greater than the number of layers in the image %d", *ll, len(p.manifest.Descriptors))
			}
			p.manifest.Descriptors = p.manifest.Descriptors[:*ll]
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
		k, err := mainManifestKey(ctx, desc, p.Platform, p.layerLimit)
		if err != nil {
			return nil, err
		}
		p.manifestKey = k.String()

		dt, err := content.ReadBlob(ctx, p.ContentStore, p.manifest.ConfigDesc)
		if err != nil {
			return nil, err
		}
		ck, err := cacheKeyFromConfig(dt, p.layerLimit)
		if err != nil {
			return nil, err
		}
		p.configKey = ck.String()
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
	var getResolver pull.SessionResolver
	switch p.ResolverType {
	case ResolverTypeRegistry:
		resolver := resolver.DefaultPool.GetResolver(p.RegistryHosts, p.Ref, "pull", p.SessionManager, g).WithImageStore(p.ImageStore, p.Mode)
		p.Puller.Resolver = resolver
		getResolver = func(g session.Group) remotes.Resolver { return resolver.WithSession(g) }
	case ResolverTypeOCILayout:
		resolver := getOCILayoutResolver(p.store, p.SessionManager, g)
		p.Puller.Resolver = resolver
		// OCILayout has no need for session
		getResolver = func(g session.Group) remotes.Resolver { return resolver }
	default:
	}

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

			if _, err := p.PullManifests(ctx, getResolver); err != nil {
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

	if p.RecordType != "" && current.GetRecordType() == "" {
		if err := current.SetRecordType(p.RecordType); err != nil {
			return nil, err
		}
	}

	return current, nil
}

// cacheKeyFromConfig returns a stable digest from image config. If image config
// is a known oci image we will use chainID of layers.
func cacheKeyFromConfig(dt []byte, layerLimit *int) (digest.Digest, error) {
	var img ocispecs.Image
	err := json.Unmarshal(dt, &img)
	if err != nil {
		if layerLimit != nil {
			return "", errors.Wrap(err, "failed to parse image config")
		}
		return digest.FromBytes(dt), nil // digest of config
	}
	if layerLimit != nil {
		l := *layerLimit
		if len(img.RootFS.DiffIDs) < l {
			return "", errors.Errorf("image has %d layers, limit is %d", len(img.RootFS.DiffIDs), l)
		}
		img.RootFS.DiffIDs = img.RootFS.DiffIDs[:l]
	}
	if img.RootFS.Type != "layers" || len(img.RootFS.DiffIDs) == 0 {
		return "", nil
	}

	return identity.ChainID(img.RootFS.DiffIDs), nil
}
