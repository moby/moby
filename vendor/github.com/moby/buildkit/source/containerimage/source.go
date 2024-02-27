package containerimage

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/pull"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
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
	g flightcontrol.Group[*resolveImageResult]
}

var _ source.Source = &Source{}

func NewSource(opt SourceOpt) (*Source, error) {
	is := &Source{
		SourceOpt: opt,
	}

	return is, nil
}

func (is *Source) Schemes() []string {
	if is.ResolverType == ResolverTypeOCILayout {
		return []string{srctypes.OCIScheme}
	}
	return []string{srctypes.DockerImageScheme}
}

func (is *Source) Identifier(scheme, ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	if is.ResolverType == ResolverTypeOCILayout {
		return is.ociIdentifier(ref, attrs, platform)
	}

	return is.registryIdentifier(ref, attrs, platform)
}

func (is *Source) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, vtx solver.Vertex) (source.SourceInstance, error) {
	var (
		p          *puller
		platform   = platforms.DefaultSpec()
		pullerUtil *pull.Puller
		mode       resolver.ResolveMode
		recordType client.UsageRecordType
		ref        reference.Spec
		store      sourceresolver.ResolveImageConfigOptStore
		layerLimit *int
	)
	switch is.ResolverType {
	case ResolverTypeRegistry:
		imageIdentifier, ok := id.(*ImageIdentifier)
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
		ociIdentifier, ok := id.(*OCIIdentifier)
		if !ok {
			return nil, errors.Errorf("invalid OCI layout identifier %v", id)
		}

		if ociIdentifier.Platform != nil {
			platform = *ociIdentifier.Platform
		}
		mode = resolver.ResolveModeForcePull // with OCI layout, we always just "pull"
		store = sourceresolver.ResolveImageConfigOptStore{
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

func (is *Source) ResolveImageConfig(ctx context.Context, ref string, opt sourceresolver.Opt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	key := ref
	var (
		rm    resolver.ResolveMode
		rslvr remotes.Resolver
		err   error
	)
	if platform := opt.Platform; platform != nil {
		key += platforms.Format(*platform)
	}

	switch is.ResolverType {
	case ResolverTypeRegistry:
		iopt := opt.ImageOpt
		if iopt == nil {
			return "", nil, errors.Errorf("missing imageopt for resolve")
		}
		rm, err = resolver.ParseImageResolveMode(iopt.ResolveMode)
		if err != nil {
			return "", nil, err
		}
		rslvr = resolver.DefaultPool.GetResolver(is.RegistryHosts, ref, "pull", sm, g).WithImageStore(is.ImageStore, rm)
	case ResolverTypeOCILayout:
		iopt := opt.OCILayoutOpt
		if iopt == nil {
			return "", nil, errors.Errorf("missing ocilayoutopt for resolve")
		}
		rm = resolver.ResolveModeForcePull
		rslvr = getOCILayoutResolver(iopt.Store, sm, g)
	}
	key += rm.String()
	res, err := is.g.Do(ctx, key, func(ctx context.Context) (*resolveImageResult, error) {
		dgst, dt, err := imageutil.Config(ctx, ref, rslvr, is.ContentStore, is.LeaseManager, opt.Platform)
		if err != nil {
			return nil, err
		}
		return &resolveImageResult{dgst: dgst, dt: dt}, nil
	})
	if err != nil {
		return "", nil, err
	}
	return res.dgst, res.dt, nil
}

type resolveImageResult struct {
	dgst digest.Digest
	dt   []byte
}

func (is *Source) registryIdentifier(ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	id, err := NewImageIdentifier(ref)
	if err != nil {
		return nil, err
	}

	if platform != nil {
		id.Platform = &ocispecs.Platform{
			OS:           platform.OS,
			Architecture: platform.Architecture,
			Variant:      platform.Variant,
			OSVersion:    platform.OSVersion,
		}
		if platform.OSFeatures != nil {
			id.Platform.OSFeatures = append([]string{}, platform.OSFeatures...)
		}
	}

	for k, v := range attrs {
		switch k {
		case pb.AttrImageResolveMode:
			rm, err := resolver.ParseImageResolveMode(v)
			if err != nil {
				return nil, err
			}
			id.ResolveMode = rm
		case pb.AttrImageRecordType:
			rt, err := parseImageRecordType(v)
			if err != nil {
				return nil, err
			}
			id.RecordType = rt
		case pb.AttrImageLayerLimit:
			l, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid layer limit %s", v)
			}
			if l <= 0 {
				return nil, errors.Errorf("invalid layer limit %s", v)
			}
			id.LayerLimit = &l
		}
	}

	return id, nil
}

func (is *Source) ociIdentifier(ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	id, err := NewOCIIdentifier(ref)
	if err != nil {
		return nil, err
	}

	if platform != nil {
		id.Platform = &ocispecs.Platform{
			OS:           platform.OS,
			Architecture: platform.Architecture,
			Variant:      platform.Variant,
			OSVersion:    platform.OSVersion,
		}
		if platform.OSFeatures != nil {
			id.Platform.OSFeatures = append([]string{}, platform.OSFeatures...)
		}
	}

	for k, v := range attrs {
		switch k {
		case pb.AttrOCILayoutSessionID:
			id.SessionID = v
		case pb.AttrOCILayoutStoreID:
			id.StoreID = v
		case pb.AttrOCILayoutLayerLimit:
			l, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid layer limit %s", v)
			}
			if l <= 0 {
				return nil, errors.Errorf("invalid layer limit %s", v)
			}
			id.LayerLimit = &l
		}
	}

	return id, nil
}

func parseImageRecordType(v string) (client.UsageRecordType, error) {
	switch client.UsageRecordType(v) {
	case "", client.UsageRecordTypeRegular:
		return client.UsageRecordTypeRegular, nil
	case client.UsageRecordTypeInternal:
		return client.UsageRecordTypeInternal, nil
	case client.UsageRecordTypeFrontend:
		return client.UsageRecordTypeFrontend, nil
	default:
		return "", errors.Errorf("invalid record type %s", v)
	}
}
