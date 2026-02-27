package containerblob

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/pkg/errors"
)

type SourceOpt struct {
	ContentStore  content.Store
	CacheAccessor cache.Accessor
	RegistryHosts docker.RegistryHosts
}

type Source struct {
	SourceOpt
}

var _ source.Source = &Source{}

func NewSource(opt SourceOpt) (*Source, error) {
	return &Source{SourceOpt: opt}, nil
}

func (is *Source) Schemes() []string {
	return []string{srctypes.DockerImageBlobScheme, srctypes.OCIBlobScheme}
}

func (is *Source) Identifier(scheme, ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	switch scheme {
	case srctypes.DockerImageBlobScheme:
		return is.registryIdentifier(ref, attrs, platform)
	case srctypes.OCIBlobScheme:
		return is.ociLayoutIdentifier(ref, attrs, platform)
	default:
		return nil, errors.Errorf("invalid image blob scheme %s", scheme)
	}
}

func (is *Source) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, vtx solver.Vertex) (source.SourceInstance, error) {
	imageIdentifier, ok := id.(*ImageBlobIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid image blob identifier %v", id)
	}
	return &puller{
		src:            is,
		id:             imageIdentifier,
		SessionManager: sm,
	}, nil
}

func (is *Source) registryIdentifier(ref string, attrs map[string]string, _ *pb.Platform) (source.Identifier, error) {
	id, err := NewImageBlobIdentifier(ref, srctypes.DockerImageBlobScheme)
	if err != nil {
		return nil, err
	}
	return is.parseIdentifierAttrs(id, attrs, false)
}

func (is *Source) ociLayoutIdentifier(ref string, attrs map[string]string, _ *pb.Platform) (source.Identifier, error) {
	id, err := NewImageBlobIdentifier(ref, srctypes.OCIBlobScheme)
	if err != nil {
		return nil, err
	}
	parsed, err := is.parseIdentifierAttrs(id, attrs, true)
	if err != nil {
		return nil, err
	}
	if id.StoreID == "" {
		return nil, errors.Errorf("oci-layout blob source requires store id")
	}
	return parsed, nil
}

func (is *Source) parseIdentifierAttrs(id *ImageBlobIdentifier, attrs map[string]string, allowOCIStore bool) (source.Identifier, error) {
	for k, v := range attrs {
		switch k {
		case pb.AttrHTTPFilename:
			id.Filename = v
		case pb.AttrHTTPPerm:
			i, err := strconv.ParseInt(v, 0, 64)
			if err != nil {
				return nil, err
			}
			id.Perm = int(i)
		case pb.AttrHTTPUID:
			i, err := strconv.ParseInt(v, 0, 64)
			if err != nil {
				return nil, err
			}
			id.UID = int(i)
		case pb.AttrHTTPGID:
			i, err := strconv.ParseInt(v, 0, 64)
			if err != nil {
				return nil, err
			}
			id.GID = int(i)
		case pb.AttrImageRecordType:
			rt, err := parseImageRecordType(v)
			if err != nil {
				return nil, err
			}
			id.RecordType = rt
		case pb.AttrOCILayoutSessionID:
			if allowOCIStore {
				id.SessionID = v
			}
		case pb.AttrOCILayoutStoreID:
			if allowOCIStore {
				id.StoreID = v
			}
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
