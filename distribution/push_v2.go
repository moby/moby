package distribution

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	distreference "github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/image"
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
	v2MetadataService *metadata.V2MetadataService
	ref               reference.Named
	endpoint          registry.APIEndpoint
	repoInfo          *registry.RepositoryInfo
	config            *ImagePushConfig
	repo              distribution.Repository

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
	if namedTagged, isNamedTagged := p.ref.(reference.NamedTagged); isNamedTagged {
		imageID, err := p.config.ReferenceStore.Get(p.ref)
		if err != nil {
			return fmt.Errorf("tag does not exist: %s", p.ref.String())
		}

		return p.pushV2Tag(ctx, namedTagged, imageID)
	}

	if !reference.IsNameOnly(p.ref) {
		return errors.New("cannot push a digest reference")
	}

	// Pull all tags
	pushed := 0
	for _, association := range p.config.ReferenceStore.ReferencesByName(p.ref) {
		if namedTagged, isNamedTagged := association.Ref.(reference.NamedTagged); isNamedTagged {
			pushed++
			if err := p.pushV2Tag(ctx, namedTagged, association.ImageID); err != nil {
				return err
			}
		}
	}

	if pushed == 0 {
		return fmt.Errorf("no tags to push for %s", p.repoInfo.Name())
	}

	return nil
}

func (p *v2Pusher) pushV2Tag(ctx context.Context, ref reference.NamedTagged, imageID image.ID) error {
	logrus.Debugf("Pushing repository: %s", ref.String())

	img, err := p.config.ImageStore.Get(imageID)
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
		v2MetadataService: p.v2MetadataService,
		repoInfo:          p.repoInfo,
		repo:              p.repo,
		pushState:         &p.pushState,
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

	// Try schema2 first
	builder := schema2.NewManifestBuilder(p.repo.Blobs(ctx), img.RawJSON())
	manifest, err := manifestFromBuilder(ctx, builder, descriptors)
	if err != nil {
		return err
	}

	manSvc, err := p.repo.Manifests(ctx)
	if err != nil {
		return err
	}

	putOptions := []distribution.ManifestServiceOption{client.WithTag(ref.Tag())}
	if _, err = manSvc.Put(ctx, manifest, putOptions...); err != nil {
		logrus.Warnf("failed to upload schema2 manifest: %v - falling back to schema1", err)

		builder = schema1.NewConfigManifestBuilder(p.repo.Blobs(ctx), p.config.TrustKey, p.repo.Name(), ref.Tag(), img.RawJSON())
		manifest, err = manifestFromBuilder(ctx, builder, descriptors)
		if err != nil {
			return err
		}

		if _, err = manSvc.Put(ctx, manifest, putOptions...); err != nil {
			return err
		}
	}

	var canonicalManifest []byte

	switch v := manifest.(type) {
	case *schema1.SignedManifest:
		canonicalManifest = v.Canonical
	case *schema2.DeserializedManifest:
		_, canonicalManifest, err = v.Payload()
		if err != nil {
			return err
		}
	}

	manifestDigest := digest.FromBytes(canonicalManifest)
	progress.Messagef(p.config.ProgressOutput, "", "%s: digest: %s size: %d", ref.Tag(), manifestDigest, len(canonicalManifest))
	// Signal digest to the trust client so it can sign the
	// push, if appropriate.
	progress.Aux(p.config.ProgressOutput, PushResult{Tag: ref.Tag(), Digest: manifestDigest, Size: len(canonicalManifest)})

	return nil
}

func manifestFromBuilder(ctx context.Context, builder distribution.ManifestBuilder, descriptors []xfer.UploadDescriptor) (distribution.Manifest, error) {
	// descriptors is in reverse order; iterate backwards to get references
	// appended in the right order.
	for i := len(descriptors) - 1; i >= 0; i-- {
		if err := builder.AppendReference(descriptors[i].(*v2PushDescriptor)); err != nil {
			return nil, err
		}
	}

	return builder.Build(ctx)
}

type v2PushDescriptor struct {
	layer             layer.Layer
	v2MetadataService *metadata.V2MetadataService
	repoInfo          reference.Named
	repo              distribution.Repository
	pushState         *pushState
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

	// Do we have any metadata associated with this layer's DiffID?
	v2Metadata, err := pd.v2MetadataService.GetMetadata(diffID)
	if err == nil {
		descriptor, exists, err := layerAlreadyExists(ctx, v2Metadata, pd.repoInfo, pd.repo, pd.pushState)
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

	var mountFrom metadata.V2Metadata

	// Attempt to find another repository in the same registry to mount the layer from to avoid an unnecessary upload
	for _, metadata := range v2Metadata {
		sourceRepo, err := reference.ParseNamed(metadata.SourceRepository)
		if err != nil {
			continue
		}
		if pd.repoInfo.Hostname() == sourceRepo.Hostname() {
			logrus.Debugf("attempting to mount layer %s (%s) from %s", diffID, metadata.Digest, sourceRepo.FullName())
			mountFrom = metadata
			break
		}
	}

	var createOpts []distribution.BlobCreateOption

	if mountFrom.SourceRepository != "" {
		namedRef, err := reference.WithName(mountFrom.SourceRepository)
		if err != nil {
			return err
		}

		// TODO (brianbland): We need to construct a reference where the Name is
		// only the full remote name, so clean this up when distribution has a
		// richer reference package
		remoteRef, err := distreference.WithName(namedRef.RemoteName())
		if err != nil {
			return err
		}

		canonicalRef, err := distreference.WithDigest(remoteRef, mountFrom.Digest)
		if err != nil {
			return err
		}

		createOpts = append(createOpts, client.WithMountFrom(canonicalRef))
	}

	// Send the layer
	layerUpload, err := bs.Create(ctx, createOpts...)
	switch err := err.(type) {
	case distribution.ErrBlobMounted:
		progress.Updatef(progressOutput, pd.ID(), "Mounted from %s", err.From.Name())

		pd.pushState.Lock()
		pd.pushState.confirmedV2 = true
		pd.pushState.remoteLayers[diffID] = err.Descriptor
		pd.pushState.Unlock()

		// Cache mapping from this layer's DiffID to the blobsum
		if err := pd.v2MetadataService.Add(diffID, metadata.V2Metadata{Digest: mountFrom.Digest, SourceRepository: pd.repoInfo.FullName()}); err != nil {
			return xfer.DoNotRetry{Err: err}
		}

		return nil
	}
	if mountFrom.SourceRepository != "" {
		// unable to mount layer from this repository, so this source mapping is no longer valid
		logrus.Debugf("unassociating layer %s (%s) with %s", diffID, mountFrom.Digest, mountFrom.SourceRepository)
		pd.v2MetadataService.Remove(mountFrom)
	}

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
	if err := pd.v2MetadataService.Add(diffID, metadata.V2Metadata{Digest: pushDigest, SourceRepository: pd.repoInfo.FullName()}); err != nil {
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

// layerAlreadyExists checks if the registry already know about any of the
// metadata passed in the "metadata" slice. If it finds one that the registry
// knows about, it returns the known digest and "true".
func layerAlreadyExists(ctx context.Context, metadata []metadata.V2Metadata, repoInfo reference.Named, repo distribution.Repository, pushState *pushState) (distribution.Descriptor, bool, error) {
	for _, meta := range metadata {
		// Only check blobsums that are known to this repository or have an unknown source
		if meta.SourceRepository != "" && meta.SourceRepository != repoInfo.FullName() {
			continue
		}
		descriptor, err := repo.Blobs(ctx).Stat(ctx, meta.Digest)
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
