package registry

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/resolver"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
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
	attrRef           = "ref"
	attrOCIMediatypes = "oci-mediatypes"
)

func ResolveCacheExporterFunc(sm *session.Manager, hosts docker.RegistryHosts) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
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
		pusher, err := remote.Pusher(ctx, ref)
		if err != nil {
			return nil, err
		}
		return remotecache.NewExporter(contentutil.FromPusher(pusher), ociMediatypes), nil
	}
}

func ResolveCacheImporterFunc(sm *session.Manager, cs content.Store, hosts docker.RegistryHosts) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Importer, specs.Descriptor, error) {
		ref, err := canonicalizeRef(attrs[attrRef])
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		remote := resolver.DefaultPool.GetResolver(hosts, ref, "pull", sm, g)
		xref, desc, err := remote.Resolve(ctx, ref)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		fetcher, err := remote.Fetcher(ctx, xref)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		src := &withDistributionSourceLabel{
			Provider: contentutil.FromFetcher(fetcher),
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
	_, err = hf(ctx, specs.Descriptor{Digest: dgst})
	return err
}

func (dsl *withDistributionSourceLabel) SetDistributionSourceAnnotation(desc specs.Descriptor) specs.Descriptor {
	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	desc.Annotations["containerd.io/distribution.source.ref"] = dsl.ref
	return desc
}
