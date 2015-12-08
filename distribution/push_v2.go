package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult struct {
	Tag    string
	Digest digest.Digest
	Size   int
}

type v2Pusher struct {
	blobSumService *metadata.BlobSumService
	ref            reference.Named
	endpoint       registry.APIEndpoint
	repoInfo       *registry.RepositoryInfo
	config         *ImagePushConfig
	repo           distribution.Repository

	// confirmedV2 is set to true if we confirm we're talking to a v2
	// registry. This is used to limit fallbacks to the v1 protocol.
	confirmedV2 bool

	// layersPushed is the set of layers known to exist on the remote side.
	// This avoids redundant queries when pushing multiple tags that
	// involve the same layers.
	layersPushed pushMap
}

type pushMap struct {
	sync.Mutex
	layersPushed map[digest.Digest]bool
}

func (p *v2Pusher) Push(ctx context.Context) (err error) {
	p.repo, p.confirmedV2, err = NewV2Repository(ctx, p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "push", "pull")
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return fallbackError{err: err, confirmedV2: p.confirmedV2}
	}

	if err = p.pushV2Repository(ctx); err != nil {
		if registry.ContinueOnError(err) {
			return fallbackError{err: err, confirmedV2: p.confirmedV2}
		}
	}
	return err
}

func (p *v2Pusher) pushV2Repository(ctx context.Context) (err error) {
	var associations []reference.Association
	if _, isTagged := p.ref.(reference.NamedTagged); isTagged {
		imageID, err := p.config.ReferenceStore.Get(p.ref)
		if err != nil {
			return fmt.Errorf("tag does not exist: %s", p.ref.String())
		}

		associations = []reference.Association{
			{
				Ref:     p.ref,
				ImageID: imageID,
			},
		}
	} else {
		// Pull all tags
		associations = p.config.ReferenceStore.ReferencesByName(p.ref)
	}
	if err != nil {
		return fmt.Errorf("error getting tags for %s: %s", p.repoInfo.Name(), err)
	}
	if len(associations) == 0 {
		return fmt.Errorf("no tags to push for %s", p.repoInfo.Name())
	}

	for _, association := range associations {
		if err := p.pushV2Tag(ctx, association); err != nil {
			return err
		}
	}

	return nil
}

func (p *v2Pusher) pushV2Tag(ctx context.Context, association reference.Association) error {
	ref := association.Ref
	logrus.Debugf("Pushing repository: %s", ref.String())

	img, err := p.config.ImageStore.Get(association.ImageID)
	if err != nil {
		return fmt.Errorf("could not find image from tag %s: %v", ref.String(), err)
	}

	var l layer.Layer

	topLayerID := img.RootFS.ChainID()
	if topLayerID == "" {
		l = layer.EmptyLayer
	} else {
		l, err = p.config.LayerStore.Get(topLayerID)
		if err != nil {
			return fmt.Errorf("failed to get top layer from image: %v", err)
		}
		defer layer.ReleaseAndLog(p.config.LayerStore, l)
	}

	var descriptors []xfer.UploadDescriptor

	descriptorTemplate := v2PushDescriptor{
		blobSumService: p.blobSumService,
		repo:           p.repo,
		layersPushed:   &p.layersPushed,
		confirmedV2:    &p.confirmedV2,
	}

	// Push empty layer if necessary
	for _, h := range img.History {
		if h.EmptyLayer {
			descriptor := descriptorTemplate
			descriptor.layer = layer.EmptyLayer
			descriptors = []xfer.UploadDescriptor{&descriptor}
			break
		}
	}

	// Loop bounds condition is to avoid pushing the base layer on Windows.
	for i := 0; i < len(img.RootFS.DiffIDs); i++ {
		descriptor := descriptorTemplate
		descriptor.layer = l
		descriptors = append(descriptors, &descriptor)

		l = l.Parent()
	}

	fsLayers, err := p.config.UploadManager.Upload(ctx, descriptors, p.config.ProgressOutput)
	if err != nil {
		return err
	}

	var tag string
	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		tag = tagged.Tag()
	}
	m, err := CreateV2Manifest(p.repo.Name(), tag, img, fsLayers)
	if err != nil {
		return err
	}

	logrus.Infof("Signed manifest for %s using daemon's key: %s", ref.String(), p.config.TrustKey.KeyID())
	signed, err := schema1.Sign(m, p.config.TrustKey)
	if err != nil {
		return err
	}

	manifestDigest, manifestSize, err := digestFromManifest(signed, ref)
	if err != nil {
		return err
	}
	if manifestDigest != "" {
		if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
			progress.Messagef(p.config.ProgressOutput, "", "%s: digest: %s size: %d", tagged.Tag(), manifestDigest, manifestSize)
			// Signal digest to the trust client so it can sign the
			// push, if appropriate.
			progress.Aux(p.config.ProgressOutput, PushResult{Tag: tagged.Tag(), Digest: manifestDigest, Size: manifestSize})
		}
	}

	manSvc, err := p.repo.Manifests(ctx)
	if err != nil {
		return err
	}
	_, err = manSvc.Put(ctx, signed)
	// FIXME create a tag
	return err
}

type v2PushDescriptor struct {
	layer          layer.Layer
	blobSumService *metadata.BlobSumService
	repo           distribution.Repository
	layersPushed   *pushMap
	confirmedV2    *bool
}

func (pd *v2PushDescriptor) Key() string {
	return "v2push:" + pd.repo.Name() + " " + pd.layer.DiffID().String()
}

func (pd *v2PushDescriptor) ID() string {
	return stringid.TruncateID(pd.layer.DiffID().String())
}

func (pd *v2PushDescriptor) DiffID() layer.DiffID {
	return pd.layer.DiffID()
}

func (pd *v2PushDescriptor) Upload(ctx context.Context, progressOutput progress.Output) (digest.Digest, error) {
	diffID := pd.DiffID()

	logrus.Debugf("Pushing layer: %s", diffID)

	// Do we have any blobsums associated with this layer's DiffID?
	possibleBlobsums, err := pd.blobSumService.GetBlobSums(diffID)
	if err == nil {
		dgst, exists, err := blobSumAlreadyExists(ctx, possibleBlobsums, pd.repo, pd.layersPushed)
		if err != nil {
			progress.Update(progressOutput, pd.ID(), "Image push failed")
			return "", retryOnError(err)
		}
		if exists {
			progress.Update(progressOutput, pd.ID(), "Layer already exists")
			return dgst, nil
		}
	}

	// if digest was empty or not saved, or if blob does not exist on the remote repository,
	// then push the blob.
	bs := pd.repo.Blobs(ctx)

	// Send the layer
	layerUpload, err := bs.Create(ctx)
	if err != nil {
		return "", retryOnError(err)
	}
	defer layerUpload.Close()

	arch, err := pd.layer.TarStream()
	if err != nil {
		return "", xfer.DoNotRetry{Err: err}
	}

	// don't care if this fails; best effort
	size, _ := pd.layer.DiffSize()

	reader := progress.NewProgressReader(ioutils.NewCancelReadCloser(ctx, arch), progressOutput, size, pd.ID(), "Pushing")
	defer reader.Close()
	compressedReader := compress(reader)

	digester := digest.Canonical.New()
	tee := io.TeeReader(compressedReader, digester.Hash())

	nn, err := layerUpload.ReadFrom(tee)
	compressedReader.Close()
	if err != nil {
		return "", retryOnError(err)
	}

	pushDigest := digester.Digest()
	if _, err := layerUpload.Commit(ctx, distribution.Descriptor{Digest: pushDigest}); err != nil {
		return "", retryOnError(err)
	}

	// If Commit succeded, that's an indication that the remote registry
	// speaks the v2 protocol.
	*pd.confirmedV2 = true

	logrus.Debugf("uploaded layer %s (%s), %d bytes", diffID, pushDigest, nn)
	progress.Update(progressOutput, pd.ID(), "Pushed")

	// Cache mapping from this layer's DiffID to the blobsum
	if err := pd.blobSumService.Add(diffID, pushDigest); err != nil {
		return "", xfer.DoNotRetry{Err: err}
	}

	pd.layersPushed.Lock()
	pd.layersPushed.layersPushed[pushDigest] = true
	pd.layersPushed.Unlock()

	return pushDigest, nil
}

// blobSumAlreadyExists checks if the registry already know about any of the
// blobsums passed in the "blobsums" slice. If it finds one that the registry
// knows about, it returns the known digest and "true".
func blobSumAlreadyExists(ctx context.Context, blobsums []digest.Digest, repo distribution.Repository, layersPushed *pushMap) (digest.Digest, bool, error) {
	layersPushed.Lock()
	for _, dgst := range blobsums {
		if layersPushed.layersPushed[dgst] {
			// it is already known that the push is not needed and
			// therefore doing a stat is unnecessary
			layersPushed.Unlock()
			return dgst, true, nil
		}
	}
	layersPushed.Unlock()

	for _, dgst := range blobsums {
		_, err := repo.Blobs(ctx).Stat(ctx, dgst)
		switch err {
		case nil:
			return dgst, true, nil
		case distribution.ErrBlobUnknown:
			// nop
		default:
			return "", false, err
		}
	}
	return "", false, nil
}

// CreateV2Manifest creates a V2 manifest from an image config and set of
// FSLayer digests.
// FIXME: This should be moved to the distribution repo, since it will also
// be useful for converting new manifests to the old format.
func CreateV2Manifest(name, tag string, img *image.Image, fsLayers map[layer.DiffID]digest.Digest) (*schema1.Manifest, error) {
	if len(img.History) == 0 {
		return nil, errors.New("empty history when trying to create V2 manifest")
	}

	// Generate IDs for each layer
	// For non-top-level layers, create fake V1Compatibility strings that
	// fit the format and don't collide with anything else, but don't
	// result in runnable images on their own.
	type v1Compatibility struct {
		ID              string    `json:"id"`
		Parent          string    `json:"parent,omitempty"`
		Comment         string    `json:"comment,omitempty"`
		Created         time.Time `json:"created"`
		ContainerConfig struct {
			Cmd []string
		} `json:"container_config,omitempty"`
		ThrowAway bool `json:"throwaway,omitempty"`
	}

	fsLayerList := make([]schema1.FSLayer, len(img.History))
	history := make([]schema1.History, len(img.History))

	parent := ""
	layerCounter := 0
	for i, h := range img.History {
		if i == len(img.History)-1 {
			break
		}

		var diffID layer.DiffID
		if h.EmptyLayer {
			diffID = layer.EmptyLayer.DiffID()
		} else {
			if len(img.RootFS.DiffIDs) <= layerCounter {
				return nil, errors.New("too many non-empty layers in History section")
			}
			diffID = img.RootFS.DiffIDs[layerCounter]
			layerCounter++
		}

		fsLayer, present := fsLayers[diffID]
		if !present {
			return nil, fmt.Errorf("missing layer in CreateV2Manifest: %s", diffID.String())
		}
		dgst := digest.FromBytes([]byte(fsLayer.Hex() + " " + parent))
		v1ID := dgst.Hex()

		v1Compatibility := v1Compatibility{
			ID:      v1ID,
			Parent:  parent,
			Comment: h.Comment,
			Created: h.Created,
		}
		v1Compatibility.ContainerConfig.Cmd = []string{img.History[i].CreatedBy}
		if h.EmptyLayer {
			v1Compatibility.ThrowAway = true
		}
		jsonBytes, err := json.Marshal(&v1Compatibility)
		if err != nil {
			return nil, err
		}

		reversedIndex := len(img.History) - i - 1
		history[reversedIndex].V1Compatibility = string(jsonBytes)
		fsLayerList[reversedIndex] = schema1.FSLayer{BlobSum: fsLayer}

		parent = v1ID
	}

	latestHistory := img.History[len(img.History)-1]

	var diffID layer.DiffID
	if latestHistory.EmptyLayer {
		diffID = layer.EmptyLayer.DiffID()
	} else {
		if len(img.RootFS.DiffIDs) <= layerCounter {
			return nil, errors.New("too many non-empty layers in History section")
		}
		diffID = img.RootFS.DiffIDs[layerCounter]
	}
	fsLayer, present := fsLayers[diffID]
	if !present {
		return nil, fmt.Errorf("missing layer in CreateV2Manifest: %s", diffID.String())
	}

	fsLayerList[0] = schema1.FSLayer{BlobSum: fsLayer}
	dgst := digest.FromBytes([]byte(fsLayer.Hex() + " " + parent + " " + string(img.RawJSON())))

	// Top-level v1compatibility string should be a modified version of the
	// image config.
	transformedConfig, err := v1.MakeV1ConfigFromConfig(img, dgst.Hex(), parent, latestHistory.EmptyLayer)
	if err != nil {
		return nil, err
	}

	history[0].V1Compatibility = string(transformedConfig)

	// windows-only baselayer setup
	if err := setupBaseLayer(history, *img.RootFS); err != nil {
		return nil, err
	}

	return &schema1.Manifest{
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
		Name:         name,
		Tag:          tag,
		Architecture: img.Architecture,
		FSLayers:     fsLayerList,
		History:      history,
	}, nil
}
