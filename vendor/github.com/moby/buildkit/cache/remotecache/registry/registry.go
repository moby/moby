package registry

import (
	"context"
	"maps"
	"strconv"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/snapshotters"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/estargz"
	"github.com/moby/buildkit/util/push"
	"github.com/moby/buildkit/util/resolver"
	resolverconfig "github.com/moby/buildkit/util/resolver/config"
	"github.com/moby/buildkit/util/resolver/limited"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func canonicalizeRef(rawRef string) (reference.Named, error) {
	if rawRef == "" {
		return nil, errors.New("missing ref")
	}
	parsed, err := reference.ParseNormalizedNamed(rawRef)
	if err != nil {
		return nil, err
	}
	parsed = reference.TagNameOnly(parsed)
	return parsed, nil
}

const (
	attrRef           = "ref"
	attrImageManifest = "image-manifest"
	attrOCIMediatypes = "oci-mediatypes"
	attrInsecure      = "registry.insecure"
)

type exporter struct {
	remotecache.Exporter
}

func (*exporter) Name() string {
	return "exporting cache to registry"
}

func ResolveCacheExporterFunc(sm *session.Manager, hosts docker.RegistryHosts) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
		compressionConfig, err := compression.ParseAttributes(attrs)
		if err != nil {
			return nil, err
		}
		ref, err := canonicalizeRef(attrs[attrRef])
		if err != nil {
			return nil, err
		}
		refString := ref.String()
		ociMediatypes := true
		if v, ok := attrs[attrOCIMediatypes]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrOCIMediatypes)
			}
			ociMediatypes = b
		}
		imageManifest := true
		if v, ok := attrs[attrImageManifest]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrImageManifest)
			}
			imageManifest = b
		} else if !ociMediatypes {
			imageManifest = false
		}
		insecure := false
		if v, ok := attrs[attrInsecure]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrInsecure)
			}
			insecure = b
		}

		scope, hosts := registryConfig(hosts, ref, resolver.ScopeType{Push: true}, insecure)
		remote := resolver.DefaultPool.GetResolver(hosts, refString, scope, sm, g)
		pusher, err := push.Pusher(ctx, remote, refString)
		if err != nil {
			return nil, err
		}
		return &exporter{remotecache.NewExporter(contentutil.FromPusher(pusher), refString, ociMediatypes, imageManifest, compressionConfig)}, nil
	}
}

func ResolveCacheImporterFunc(sm *session.Manager, cs content.Store, hosts docker.RegistryHosts) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Importer, ocispecs.Descriptor, error) {
		ref, err := canonicalizeRef(attrs[attrRef])
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		refString := ref.String()
		insecure := false
		if v, ok := attrs[attrInsecure]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, ocispecs.Descriptor{}, errors.Wrapf(err, "failed to parse %s", attrInsecure)
			}
			insecure = b
		}

		scope, hosts := registryConfig(hosts, ref, resolver.ScopeType{}, insecure)
		remote := resolver.DefaultPool.GetResolver(hosts, refString, scope, sm, g)
		xref, desc, err := remote.Resolve(ctx, refString)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		src := &registryCacheProvider{
			resolver: remote,
			ref:      refString,
			xref:     xref,
			source:   cs,
		}
		return remotecache.NewImporter(src), desc, nil
	}
}

type registryCacheProvider struct {
	resolver *resolver.Resolver
	ref      string
	xref     string
	source   content.Manager
}

var _ remotecache.DistributionSourceLabelSetter = &registryCacheProvider{}

// ProviderForSession prevents cache providers shared across Solves from using
// the credentials of the Solve that originally imported the cache manifest.
func (p *registryCacheProvider) ProviderForSession(g session.Group) content.Provider {
	p2 := *p
	p2.resolver = p.resolver.WithSession(g)
	return &p2
}

func (p *registryCacheProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	fetcher, err := p.resolver.Fetcher(ctx, p.xref)
	if err != nil {
		return nil, err
	}
	return contentutil.FromFetcher(limited.Default.WrapFetcher(fetcher, p.ref)).ReaderAt(ctx, desc)
}

func (p *registryCacheProvider) SetDistributionSourceLabel(ctx context.Context, dgst digest.Digest) error {
	hf, err := docker.AppendDistributionSourceLabel(p.source, p.ref)
	if err != nil {
		return err
	}
	_, err = hf(ctx, ocispecs.Descriptor{Digest: dgst})
	return err
}

func (p *registryCacheProvider) SetDistributionSourceAnnotation(desc ocispecs.Descriptor) ocispecs.Descriptor {
	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	desc.Annotations["containerd.io/distribution.source.ref"] = p.ref
	return desc
}

func (p *registryCacheProvider) SnapshotLabels(descs []ocispecs.Descriptor, index int) map[string]string {
	if len(descs) < index {
		return nil
	}
	labels := snapshots.FilterInheritedLabels(descs[index].Annotations)
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, estargz.SnapshotLabels(p.ref, descs, index))
	labels[snapshotters.TargetRefLabel] = p.ref
	return labels
}

func registryConfig(hosts docker.RegistryHosts, ref reference.Named, scope resolver.ScopeType, insecure bool) (resolver.ScopeType, docker.RegistryHosts) {
	if insecure {
		insecureTrue := true
		httpTrue := true
		hosts = resolver.NewRegistryConfig(map[string]resolverconfig.RegistryConfig{
			reference.Domain(ref): {
				Insecure:  &insecureTrue,
				PlainHTTP: &httpTrue,
			},
		})
		scope.Insecure = true
	}
	return scope, hosts
}
