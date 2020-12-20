package distribution

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema1"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// This is used by manifestStore to pare down the requirements to implement a
// full distribution.ManifestService, since `Get` is all we use here.
type manifestGetter interface {
	Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error)
}

type manifestStore struct {
	local  ContentStore
	remote manifestGetter
}

// ContentStore is the interface used to persist registry blobs
//
// Currently this is only used to persist manifests and manifest lists.
// It is exported because `distribution.Pull` takes one as an argument.
type ContentStore interface {
	content.Ingester
	content.Provider
	Info(ctx context.Context, dgst digest.Digest) (content.Info, error)
	Abort(ctx context.Context, ref string) error
}

func (m *manifestStore) getLocal(ctx context.Context, desc specs.Descriptor) (distribution.Manifest, error) {
	ra, err := m.local.ReaderAt(ctx, desc)
	if err != nil {
		return nil, errors.Wrap(err, "error getting content store reader")
	}
	defer ra.Close()

	r := io.NewSectionReader(ra, 0, ra.Size())
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "error reading manifest from content store")
	}

	manifest, _, err := distribution.UnmarshalManifest(desc.MediaType, data)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshaling manifest from content store")
	}
	return manifest, nil
}

func (m *manifestStore) getMediaType(ctx context.Context, desc specs.Descriptor) (string, error) {
	ra, err := m.local.ReaderAt(ctx, desc)
	if err != nil {
		return "", errors.Wrap(err, "error getting reader to detect media type")
	}
	defer ra.Close()

	mt, err := detectManifestMediaType(ra)
	if err != nil {
		return "", errors.Wrap(err, "error detecting media type")
	}
	return mt, nil
}

func (m *manifestStore) Get(ctx context.Context, desc specs.Descriptor) (distribution.Manifest, error) {
	l := log.G(ctx)

	if desc.MediaType == "" {
		// When pulling by digest we will not have the media type on the
		// descriptor since we have not made a request to the registry yet
		//
		// We already have the digest, so we only lookup locally... by digest.
		//
		// Let's try to detect the media type so we can have a good ref key
		// here. We may not even have the content locally, and this is fine, but
		// if we do we should determine that.
		mt, err := m.getMediaType(ctx, desc)
		if err != nil && !errdefs.IsNotFound(err) {
			l.WithError(err).Warn("Error looking up media type of content")
		}
		desc.MediaType = mt
	}

	key := remotes.MakeRefKey(ctx, desc)

	// Here we open a writer to the requested content. This both gives us a
	// reference to write to if indeed we need to persist it and increments the
	// ref count on the content.
	w, err := m.local.Writer(ctx, content.WithDescriptor(desc), content.WithRef(key))
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			var manifest distribution.Manifest
			if manifest, err = m.getLocal(ctx, desc); err == nil {
				return manifest, nil
			}
		}
		// always fallback to the remote if there is an error with the local store
	}
	if w != nil {
		defer w.Close()
	}

	l.WithError(err).Debug("Fetching manifest from remote")

	manifest, err := m.remote.Get(ctx, desc.Digest)
	if err != nil {
		if err := m.local.Abort(ctx, key); err != nil {
			l.WithError(err).Warn("Error while attempting to abort content ingest")
		}
		return nil, err
	}

	if w != nil {
		// if `w` is nil here, something happened with the content store, so don't bother trying to persist.
		if err := m.Put(ctx, manifest, desc, w); err != nil {
			if err := m.local.Abort(ctx, key); err != nil {
				l.WithError(err).Warn("error aborting content ingest")
			}
			l.WithError(err).Warn("Error persisting manifest")
		}
	}
	return manifest, nil
}

func (m *manifestStore) Put(ctx context.Context, manifest distribution.Manifest, desc specs.Descriptor, w content.Writer) error {
	mt, payload, err := manifest.Payload()
	if err != nil {
		return err
	}
	desc.Size = int64(len(payload))
	desc.MediaType = mt

	if _, err = w.Write(payload); err != nil {
		return errors.Wrap(err, "error writing manifest to content store")
	}

	if err := w.Commit(ctx, desc.Size, desc.Digest); err != nil {
		return errors.Wrap(err, "error committing manifest to content store")
	}
	return nil
}

func detectManifestMediaType(ra content.ReaderAt) (string, error) {
	dt := make([]byte, ra.Size())
	if _, err := ra.ReadAt(dt, 0); err != nil {
		return "", err
	}

	return detectManifestBlobMediaType(dt)
}

// This is used when the manifest store does not know the media type of a sha it
// was told to get. This would currently only happen when pulling by digest.
// The media type is needed so the blob can be unmarshalled properly.
func detectManifestBlobMediaType(dt []byte) (string, error) {
	var mfst struct {
		MediaType string          `json:"mediaType"`
		Config    json.RawMessage `json:"config"`   // schema2 Manifest
		FSLayers  json.RawMessage `json:"fsLayers"` // schema1 Manifest
	}

	if err := json.Unmarshal(dt, &mfst); err != nil {
		return "", err
	}

	// We may have a media type specified in the json, in which case that should be used.
	// Docker types should generally have a media type set.
	// OCI (golang) types do not have a `mediaType` defined, and it is optional in the spec.
	//
	// `distrubtion.UnmarshalManifest`, which is used to unmarshal this for real, checks these media type values.
	// If the specified media type does not match it will error, and in some cases (docker media types) it is required.
	// So pretty much if we don't have a media type we can fall back to OCI.
	// This does have a special fallback for schema1 manifests just because it is easy to detect.
	switch {
	case mfst.MediaType != "":
		return mfst.MediaType, nil
	case mfst.FSLayers != nil:
		return schema1.MediaTypeManifest, nil
	case mfst.Config != nil:
		return specs.MediaTypeImageManifest, nil
	default:
		return specs.MediaTypeImageIndex, nil
	}
}
