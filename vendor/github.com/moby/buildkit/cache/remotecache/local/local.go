package local

import (
	"context"
	"strconv"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/session"
	sessioncontent "github.com/moby/buildkit/session/content"
	"github.com/moby/buildkit/util/compression"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	attrDigest           = "digest"
	attrSrc              = "src"
	attrDest             = "dest"
	attrImageManifest    = "image-manifest"
	attrOCIMediatypes    = "oci-mediatypes"
	contentStoreIDPrefix = "local:"
)

type exporter struct {
	remotecache.Exporter
}

func (*exporter) Name() string {
	return "exporting cache to client directory"
}

// ResolveCacheExporterFunc for "local" cache exporter.
func ResolveCacheExporterFunc(sm *session.Manager) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
		store := attrs[attrDest]
		if store == "" {
			return nil, errors.New("local cache exporter requires dest")
		}
		compressionConfig, err := compression.ParseAttributes(attrs)
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
		imageManifest := false
		if v, ok := attrs[attrImageManifest]; ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", attrImageManifest)
			}
			imageManifest = b
		}
		csID := contentStoreIDPrefix + store
		cs, err := getContentStore(ctx, sm, g, csID)
		if err != nil {
			return nil, err
		}
		return &exporter{remotecache.NewExporter(cs, "", ociMediatypes, imageManifest, compressionConfig)}, nil
	}
}

// ResolveCacheImporterFunc for "local" cache importer.
func ResolveCacheImporterFunc(sm *session.Manager) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Importer, ocispecs.Descriptor, error) {
		dgstStr := attrs[attrDigest]
		if dgstStr == "" {
			return nil, ocispecs.Descriptor{}, errors.New("local cache importer requires explicit digest")
		}
		dgst := digest.Digest(dgstStr)
		store := attrs[attrSrc]
		if store == "" {
			return nil, ocispecs.Descriptor{}, errors.New("local cache importer requires src")
		}
		csID := contentStoreIDPrefix + store
		cs, err := getContentStore(ctx, sm, g, csID)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		info, err := cs.Info(ctx, dgst)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		desc := ocispecs.Descriptor{
			// MediaType is typically MediaTypeDockerSchema2ManifestList,
			// but we leave it empty until we get correct support for local index.json
			Digest: dgst,
			Size:   info.Size,
		}
		return remotecache.NewImporter(cs), desc, nil
	}
}

func getContentStore(ctx context.Context, sm *session.Manager, g session.Group, storeID string) (content.Store, error) {
	// TODO: to ensure correct session is detected, new api for finding if storeID is supported is needed
	sessionID := g.SessionIterator().NextSession()
	if sessionID == "" {
		return nil, errors.New("local cache exporter/importer requires session")
	}
	timeoutCtx, cancel := context.WithCancelCause(context.Background())
	timeoutCtx, _ = context.WithTimeoutCause(timeoutCtx, 5*time.Second, errors.WithStack(context.DeadlineExceeded))
	defer cancel(errors.WithStack(context.Canceled))

	caller, err := sm.Get(timeoutCtx, sessionID, false)
	if err != nil {
		return nil, err
	}
	return &unlazyProvider{sessioncontent.NewCallerStore(caller, storeID), g}, nil
}

type unlazyProvider struct {
	content.Store
	s session.Group
}

func (p *unlazyProvider) UnlazySession(desc ocispecs.Descriptor) session.Group {
	return p.s
}
