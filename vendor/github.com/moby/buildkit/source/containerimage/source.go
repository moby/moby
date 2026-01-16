package containerimage

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/reference"
	"github.com/containerd/platforms"
	distreference "github.com/distribution/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/pull"
	"github.com/moby/buildkit/util/resolver"
	"github.com/moby/buildkit/util/tracing"
	policyimage "github.com/moby/policy-helpers/image"
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
	gImageRes    flightcontrol.Group[*resolveImageResult]
	gAttestChain flightcontrol.Group[*sourceresolver.AttestationChain]
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
		checksum   digest.Digest
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
		checksum = imageIdentifier.Checksum
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
		checksum:       checksum,
	}
	return p, nil
}

func (is *Source) ResolveImageMetadata(ctx context.Context, id *ImageIdentifier, opt *sourceresolver.ResolveImageOpt, sm *session.Manager, g session.Group) (_ *sourceresolver.ResolveImageResponse, retErr error) {
	if is.ResolverType != ResolverTypeRegistry {
		return nil, errors.Errorf("invalid resolver type for image metadata: %v", is.ResolverType)
	}
	ref := id.Reference.String()

	span, ctx := tracing.StartSpan(ctx, "resolving "+ref)
	defer func() {
		tracing.FinishWithError(span, retErr)
	}()

	key := ref
	if platform := opt.Platform; platform != nil {
		key += platforms.FormatAll(*platform)
	}
	rm, err := resolver.ParseImageResolveMode(opt.ResolveMode)
	if err != nil {
		return nil, err
	}
	rslvr := resolver.DefaultPool.GetResolver(is.RegistryHosts, ref, resolver.ScopeType{}, sm, g).WithImageStore(is.ImageStore, rm)
	key += rm.String()

	ret := &sourceresolver.ResolveImageResponse{}
	if !opt.NoConfig {
		res, err := is.gImageRes.Do(ctx, key, func(ctx context.Context) (*resolveImageResult, error) {
			dgst, dt, err := imageutil.Config(ctx, ref, rslvr, is.ContentStore, is.LeaseManager, opt.Platform)
			if err != nil {
				return nil, err
			}
			return &resolveImageResult{dgst: dgst, dt: dt}, nil
		})
		if err != nil {
			return nil, err
		}
		ret.Digest = res.dgst
		ret.Config = res.dt
	}
	if opt.AttestationChain {
		ctx, done, err := leaseutil.WithLease(ctx, is.LeaseManager, leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer func() {
			// this lease is not deleted to allow other components to access manifest/config from cache. It will be deleted after 5 min deadline or on pruning inactive builder
			imageutil.AddLease(done)
		}()
		res, err := is.gAttestChain.Do(ctx, key, func(ctx context.Context) (*sourceresolver.AttestationChain, error) {
			refStr, desc, err := rslvr.Resolve(ctx, ref)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			f, err := rslvr.Fetcher(ctx, refStr)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			named, err := distreference.ParseNormalizedNamed(ref)
			if err != nil {
				return nil, err
			}
			prov := contentutil.ReferrersProviderWithBuffer(contentutil.FromFetcher(f), is.ContentStore, named.Name())
			sc, err := policyimage.ResolveSignatureChain(ctx, prov, desc, opt.Platform)
			if err != nil {
				return nil, err
			}
			ac := &sourceresolver.AttestationChain{
				Root: desc.Digest,
			}
			descs := []ocispecs.Descriptor{desc}
			if sc.ImageManifest != nil {
				// not adding image manifest to descs as it is not really needed for verification
				// still adding digest to provide hint of what the image manifest was resolved by platform
				// for better debugging experience  and error messages
				ac.ImageManifest = sc.ImageManifest.Digest
			}
			if sc.AttestationManifest != nil {
				ac.AttestationManifest = sc.AttestationManifest.Digest
				descs = append(descs, sc.AttestationManifest.Descriptor)
			}
			if sc.SignatureManifest != nil {
				ac.SignatureManifests = []digest.Digest{sc.SignatureManifest.Digest}
				descs = append(descs, sc.SignatureManifest.Descriptor)
				mfst, err := sc.OCIManifest(ctx, sc.SignatureManifest)
				if err != nil {
					return nil, errors.WithStack(err)
				}
				descs = append(descs, mfst.Layers...)
			}
			for _, desc := range descs {
				dt, err := policyimage.ReadBlob(ctx, prov, desc)
				if err != nil {
					return nil, errors.WithStack(err)
				}
				if ac.Blobs == nil {
					ac.Blobs = make(map[digest.Digest]sourceresolver.Blob)
				}
				ac.Blobs[desc.Digest] = sourceresolver.Blob{
					Descriptor: desc,
					Data:       dt,
				}
			}
			if err := prov.SetGCLabels(ctx, desc); err != nil {
				return nil, errors.WithStack(err)
			}
			return ac, nil
		})
		if err != nil {
			return nil, err
		}
		ret.AttestationChain = res
		if ret.Digest == "" {
			ret.Digest = res.Root
		} else if ret.Digest != res.Root {
			return nil, errors.Errorf("attestation chain root digest %s does not match image digest %s", res.Root, ret.Digest)
		}
	}
	return ret, nil
}

func (is *Source) ResolveOCILayoutMetadata(ctx context.Context, id *OCIIdentifier, opt *sourceresolver.ResolveOCILayoutOpt, sm *session.Manager, g session.Group) (_ *sourceresolver.ResolveImageResponse, retErr error) {
	if is.ResolverType != ResolverTypeOCILayout {
		return nil, errors.Errorf("invalid resolver type for image metadata: %v", is.ResolverType)
	}
	ref := id.Reference.String()

	span, ctx := tracing.StartSpan(ctx, "resolving "+ref)
	defer func() {
		tracing.FinishWithError(span, retErr)
	}()

	key := ref
	if platform := opt.Platform; platform != nil {
		key += platforms.FormatAll(*platform)
	}

	if opt.Store.StoreID == "" {
		opt.Store.StoreID = id.StoreID
	}

	rslvr := getOCILayoutResolver(opt.Store, sm, g)
	key += resolver.ResolveModeForcePull.String()

	res, err := is.gImageRes.Do(ctx, key, func(ctx context.Context) (*resolveImageResult, error) {
		dgst, dt, err := imageutil.Config(ctx, ref, rslvr, is.ContentStore, is.LeaseManager, opt.Platform)
		if err != nil {
			return nil, err
		}
		return &resolveImageResult{dgst: dgst, dt: dt}, nil
	})
	if err != nil {
		return nil, err
	}
	return &sourceresolver.ResolveImageResponse{
		Digest: res.dgst,
		Config: res.dt,
	}, nil
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
			id.Platform.OSFeatures = slices.Clone(platform.OSFeatures)
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
		case pb.AttrImageChecksum:
			dgst, err := digest.Parse(v)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid image checksum %s", v)
			}
			id.Checksum = dgst
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
			id.Platform.OSFeatures = slices.Clone(platform.OSFeatures)
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
