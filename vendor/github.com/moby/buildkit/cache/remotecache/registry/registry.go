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
	"github.com/moby/buildkit/util/resolver/limited"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func canonicalizeRef(rawRef string) (string, error) {
	if rawRef == "" {
		return "", errors.New("missing ref")
	}
	parsed, err := reference.ParseNormalizedNamed(rawRef)
	if err != nil {
		return "", err
	}
	return reference.TagNameOnly(parsed).String(), nil
}

const (
	attrRef              = "ref"
	attrOCIMediatypes    = "oci-mediatypes"
	attrLayerCompression = "compression"
	attrForceCompression = "force-compression"
	attrCompressionLevel = "compression-level"
)

func ResolveCacheExporterFunc(sm *session.Manager, hosts docker.RegistryHosts) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
		compressionConfig, err := attrsToCompression(attrs)
		if err != nil {
			return nil, err
		}
		ref, err := canonicalizeRef(attrs[attrRef])
		if err != nil {
			return nil, err
		}
		ociMediatypes := true
		if v, ok := attrs[attrOCIMediatypes]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrOCIMediatypes)
			}
			ociMediatypes = b
		}
		remote := resolver.DefaultPool.GetResolver(hosts, ref, "push", sm, g)
		pusher, err := push.Pusher(ctx, remote, ref)
		if err != nil {
			return nil, err
		}
		return remotecache.NewExporter(contentutil.FromPusher(pusher), ref, ociMediatypes, *compressionConfig), nil
	}
}

func ResolveCacheImporterFunc(sm *session.Manager, cs content.Store, hosts docker.RegistryHosts) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Importer, ocispecs.Descriptor, error) {
		ref, err := canonicalizeRef(attrs[attrRef])
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		remote := resolver.DefaultPool.GetResolver(hosts, ref, "pull", sm, g)
		xref, desc, err := remote.Resolve(ctx, ref)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		fetcher, err := remote.Fetcher(ctx, xref)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		src := &withDistributionSourceLabel{
			Provider: contentutil.FromFetcher(limited.Default.WrapFetcher(fetcher, ref)),
			ref:      ref,
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

func attrsToCompression(attrs map[string]string) (*compression.Config, error) {
	compressionType := compression.Default
	if v, ok := attrs[attrLayerCompression]; ok {
		if c := compression.Parse(v); c != compression.UnknownCompression {
			compressionType = c
		}
	}
	compressionConfig := compression.New(compressionType)
	if v, ok := attrs[attrForceCompression]; ok {
		var force bool
		if v == "" {
			force = true
		} else {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "non-bool value %s specified for %s", v, attrForceCompression)
			}
			force = b
		}
		compressionConfig = compressionConfig.SetForce(force)
	}
	if v, ok := attrs[attrCompressionLevel]; ok {
		ii, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "non-integer value %s specified for %s", v, attrCompressionLevel)
		}
		compressionConfig = compressionConfig.SetLevel(int(ii))
	}
	return &compressionConfig, nil
}
