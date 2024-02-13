package registry

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/distribution/reference"
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
		imageManifest := false
		if v, ok := attrs[attrImageManifest]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrImageManifest)
			}
			imageManifest = b
		}
		insecure := false
		if v, ok := attrs[attrInsecure]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrInsecure)
			}
			insecure = b
		}

		scope, hosts := registryConfig(hosts, ref, "push", insecure)
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

		scope, hosts := registryConfig(hosts, ref, "pull", insecure)
		remote := resolver.DefaultPool.GetResolver(hosts, refString, scope, sm, g)
		xref, desc, err := remote.Resolve(ctx, refString)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		fetcher, err := remote.Fetcher(ctx, xref)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		src := &withDistributionSourceLabel{
			Provider: contentutil.FromFetcher(limited.Default.WrapFetcher(fetcher, refString)),
			ref:      refString,
			source:   cs,
		}
		return remotecache.NewImporter(src), desc, nil
	}
}

type withDistributionSourceLabel struct {
	content.Provider
	ref    string
	source content.Manager
}

var _ remotecache.DistributionSourceLabelSetter = &withDistributionSourceLabel{}

func (dsl *withDistributionSourceLabel) SetDistributionSourceLabel(ctx context.Context, dgst digest.Digest) error {
	hf, err := docker.AppendDistributionSourceLabel(dsl.source, dsl.ref)
	if err != nil {
		return err
	}
	_, err = hf(ctx, ocispecs.Descriptor{Digest: dgst})
	return err
}

func (dsl *withDistributionSourceLabel) SetDistributionSourceAnnotation(desc ocispecs.Descriptor) ocispecs.Descriptor {
	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	desc.Annotations["containerd.io/distribution.source.ref"] = dsl.ref
	return desc
}

func (dsl *withDistributionSourceLabel) SnapshotLabels(descs []ocispecs.Descriptor, index int) map[string]string {
	if len(descs) < index {
		return nil
	}
	labels := snapshots.FilterInheritedLabels(descs[index].Annotations)
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range estargz.SnapshotLabels(dsl.ref, descs, index) {
		labels[k] = v
	}
	return labels
}

func registryConfig(hosts docker.RegistryHosts, ref reference.Named, scope string, insecure bool) (string, docker.RegistryHosts) {
	if insecure {
		insecureTrue := true
		httpTrue := true
		hosts = resolver.NewRegistryConfig(map[string]resolverconfig.RegistryConfig{
			reference.Domain(ref): {
				Insecure:  &insecureTrue,
				PlainHTTP: &httpTrue,
			},
		})
		scope += ":insecure"
	}
	return scope, hosts
}
