package distribution

import (
	"fmt"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
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

	// pushState is state built by the Download functions.
	pushState pushState
}

type pushState struct {
	sync.Mutex
	// remoteLayers is the set of layers known to exist on the remote side.
	// This avoids redundant queries when pushing multiple tags that
	// involve the same layers. It is also used to fill in digest and size
	// information when building the manifest.
	remoteLayers map[layer.DiffID]distribution.Descriptor
	// confirmedV2 is set to true if we confirm we're talking to a v2
	// registry. This is used to limit fallbacks to the v1 protocol.
	confirmedV2 bool
}

func (p *v2Pusher) Push(ctx context.Context) (err error) {
	p.pushState.remoteLayers = make(map[layer.DiffID]distribution.Descriptor)

	p.repo, p.pushState.confirmedV2, err = NewV2Repository(ctx, p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "push", "pull")
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return fallbackError{err: err, confirmedV2: p.pushState.confirmedV2}
	}

	if err = p.pushV2Repository(ctx); err != nil {
		if registry.ContinueOnError(err) {
			return fallbackError{err: err, confirmedV2: p.pushState.confirmedV2}
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
		pushState:      &p.pushState,
	}

	// Loop bounds condition is to avoid pushing the base layer on Windows.
	for i := 0; i < len(img.RootFS.DiffIDs); i++ {
		descriptor := descriptorTemplate
		descriptor.layer = l
		descriptors = append(descriptors, &descriptor)

		l = l.Parent()
	}

	if err := p.config.UploadManager.Upload(ctx, descriptors, p.config.ProgressOutput); err != nil {
		return err
	}

	var tag string
	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		tag = tagged.Tag()
	}
	builder := schema1.NewConfigManifestBuilder(p.repo.Blobs(ctx), p.config.TrustKey, p.repo.Name(), tag, img.RawJSON())

	// descriptors is in reverse order; iterate backwards to get references
	// appended in the right order.
	for i := len(descriptors) - 1; i >= 0; i-- {
		if err := builder.AppendReference(descriptors[i].(*v2PushDescriptor)); err != nil {
			return err
		}
	}

	manifest, err := builder.Build(ctx)
	if err != nil {
		return err
	}

	manifestDigest, manifestSize, err := digestFromManifest(manifest.(*schema1.SignedManifest), ref)
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

	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		_, err = manSvc.Put(ctx, manifest, client.WithTag(tagged.Tag()))
	} else {
		_, err = manSvc.Put(ctx, manifest)
	}
	// FIXME create a tag
	return err
}

type v2PushDescriptor struct {
	layer          layer.Layer
	blobSumService *metadata.BlobSumService
	repo           distribution.Repository
	pushState      *pushState
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

func (pd *v2PushDescriptor) Upload(ctx context.Context, progressOutput progress.Output) error {
	diffID := pd.DiffID()

	pd.pushState.Lock()
	if _, ok := pd.pushState.remoteLayers[diffID]; ok {
		// it is already known that the push is not needed and
		// therefore doing a stat is unnecessary
		pd.pushState.Unlock()
		progress.Update(progressOutput, pd.ID(), "Layer already exists")
		return nil
	}
	pd.pushState.Unlock()

	// Do we have any blobsums associated with this layer's DiffID?
	possibleBlobsums, err := pd.blobSumService.GetBlobSums(diffID)
	if err == nil {
		descriptor, exists, err := blobSumAlreadyExists(ctx, possibleBlobsums, pd.repo, pd.pushState)
		if err != nil {
			progress.Update(progressOutput, pd.ID(), "Image push failed")
			return retryOnError(err)
		}
		if exists {
			progress.Update(progressOutput, pd.ID(), "Layer already exists")
			pd.pushState.Lock()
			pd.pushState.remoteLayers[diffID] = descriptor
			pd.pushState.Unlock()
			return nil
		}
	}

	logrus.Debugf("Pushing layer: %s", diffID)

	// if digest was empty or not saved, or if blob does not exist on the remote repository,
	// then push the blob.
	bs := pd.repo.Blobs(ctx)

	// Send the layer
	layerUpload, err := bs.Create(ctx)
	if err != nil {
		return retryOnError(err)
	}
	defer layerUpload.Close()

	arch, err := pd.layer.TarStream()
	if err != nil {
		return xfer.DoNotRetry{Err: err}
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
		return retryOnError(err)
	}

	pushDigest := digester.Digest()
	if _, err := layerUpload.Commit(ctx, distribution.Descriptor{Digest: pushDigest}); err != nil {
		return retryOnError(err)
	}

	logrus.Debugf("uploaded layer %s (%s), %d bytes", diffID, pushDigest, nn)
	progress.Update(progressOutput, pd.ID(), "Pushed")

	// Cache mapping from this layer's DiffID to the blobsum
	if err := pd.blobSumService.Add(diffID, pushDigest); err != nil {
		return xfer.DoNotRetry{Err: err}
	}

	pd.pushState.Lock()

	// If Commit succeded, that's an indication that the remote registry
	// speaks the v2 protocol.
	pd.pushState.confirmedV2 = true

	pd.pushState.remoteLayers[diffID] = distribution.Descriptor{
		Digest:    pushDigest,
		MediaType: schema2.MediaTypeLayer,
		Size:      nn,
	}

	pd.pushState.Unlock()

	return nil
}

func (pd *v2PushDescriptor) Descriptor() distribution.Descriptor {
	// Not necessary to lock pushStatus because this is always
	// called after all the mutation in pushStatus.
	// By the time this function is called, every layer will have
	// an entry in remoteLayers.
	return pd.pushState.remoteLayers[pd.DiffID()]
}

// blobSumAlreadyExists checks if the registry already know about any of the
// blobsums passed in the "blobsums" slice. If it finds one that the registry
// knows about, it returns the known digest and "true".
func blobSumAlreadyExists(ctx context.Context, blobsums []digest.Digest, repo distribution.Repository, pushState *pushState) (distribution.Descriptor, bool, error) {
	for _, dgst := range blobsums {
		descriptor, err := repo.Blobs(ctx).Stat(ctx, dgst)
		switch err {
		case nil:
			descriptor.MediaType = schema2.MediaTypeLayer
			return descriptor, true, nil
		case distribution.ErrBlobUnknown:
			// nop
		default:
			return distribution.Descriptor{}, false, err
		}
	}
	return distribution.Descriptor{}, false, nil
}
