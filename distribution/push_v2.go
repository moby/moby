package distribution

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/context"

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
)

const maxRepositoryMountAttempts = 4

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult struct {
	Tag    string
	Digest digest.Digest
	Size   int
}

type v2Pusher struct {
	v2MetadataService metadata.V2MetadataService
	ref               reference.Named
	endpoint          registry.APIEndpoint
	repoInfo          *registry.RepositoryInfo
	config            *ImagePushConfig
	repo              distribution.Repository

	// pushState is state built by the Upload functions.
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
		return err
	}

	if err = p.pushV2Repository(ctx); err != nil {
		if continueOnError(err) {
			return fallbackError{
				err:         err,
				confirmedV2: p.pushState.confirmedV2,
				transportOK: true,
			}
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

	hmacKey, err := metadata.ComputeV2MetadataHMACKey(p.config.AuthConfig)
	if err != nil {
		return fmt.Errorf("failed to compute hmac key of auth config: %v", err)
	}

	var descriptors []xfer.UploadDescriptor

	descriptorTemplate := v2PushDescriptor{
		v2MetadataService: p.v2MetadataService,
		hmacKey:           hmacKey,
		repoInfo:          p.repoInfo,
		ref:               p.ref,
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

	putOptions := []distribution.ManifestServiceOption{distribution.WithTag(ref.Tag())}
	if _, err = manSvc.Put(ctx, manifest, putOptions...); err != nil {
		if runtime.GOOS == "windows" {
			logrus.Warnf("failed to upload schema2 manifest: %v", err)
			return err
		}

		logrus.Warnf("failed to upload schema2 manifest: %v - falling back to schema1", err)

		manifestRef, err := distreference.WithTag(p.repo.Named(), ref.Tag())
		if err != nil {
			return err
		}
		builder = schema1.NewConfigManifestBuilder(p.repo.Blobs(ctx), p.config.TrustKey, manifestRef, img.RawJSON())
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

	if err := addDigestReference(p.config.ReferenceStore, ref, manifestDigest, imageID); err != nil {
		return err
	}

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
	v2MetadataService metadata.V2MetadataService
	hmacKey           []byte
	repoInfo          reference.Named
	ref               reference.Named
	repo              distribution.Repository
	pushState         *pushState
	remoteDescriptor  distribution.Descriptor
}

func (pd *v2PushDescriptor) Key() string {
	return "v2push:" + pd.ref.FullName() + " " + pd.layer.DiffID().String()
}

func (pd *v2PushDescriptor) ID() string {
	return stringid.TruncateID(pd.layer.DiffID().String())
}

func (pd *v2PushDescriptor) DiffID() layer.DiffID {
	return pd.layer.DiffID()
}

func (pd *v2PushDescriptor) Upload(ctx context.Context, progressOutput progress.Output) (distribution.Descriptor, error) {
	if fs, ok := pd.layer.(distribution.Describable); ok {
		if d := fs.Descriptor(); len(d.URLs) > 0 {
			progress.Update(progressOutput, pd.ID(), "Skipped foreign layer")
			return d, nil
		}
	}

	diffID := pd.DiffID()

	pd.pushState.Lock()
	if descriptor, ok := pd.pushState.remoteLayers[diffID]; ok {
		// it is already known that the push is not needed and
		// therefore doing a stat is unnecessary
		pd.pushState.Unlock()
		progress.Update(progressOutput, pd.ID(), "Layer already exists")
		return descriptor, nil
	}
	pd.pushState.Unlock()

	// Do we have any metadata associated with this layer's DiffID?
	v2Metadata, err := pd.v2MetadataService.GetMetadata(diffID)
	if err == nil {
		descriptor, exists, err := layerAlreadyExists(ctx, v2Metadata, pd.repoInfo, pd.repo, pd.pushState)
		if err != nil {
			progress.Update(progressOutput, pd.ID(), "Image push failed")
			return distribution.Descriptor{}, retryOnError(err)
		}
		if exists {
			progress.Update(progressOutput, pd.ID(), "Layer already exists")
			pd.pushState.Lock()
			pd.pushState.remoteLayers[diffID] = descriptor
			pd.pushState.Unlock()
			return descriptor, nil
		}
	}

	logrus.Debugf("Pushing layer: %s", diffID)

	// if digest was empty or not saved, or if blob does not exist on the remote repository,
	// then push the blob.
	bs := pd.repo.Blobs(ctx)

	var layerUpload distribution.BlobWriter

	// Attempt to find another repository in the same registry to mount the layer from to avoid an unnecessary upload
	candidates := getRepositoryMountCandidates(pd.repoInfo, pd.hmacKey, maxRepositoryMountAttempts, v2Metadata)
	for _, mountCandidate := range candidates {
		logrus.Debugf("attempting to mount layer %s (%s) from %s", diffID, mountCandidate.Digest, mountCandidate.SourceRepository)
		createOpts := []distribution.BlobCreateOption{}

		if len(mountCandidate.SourceRepository) > 0 {
			namedRef, err := reference.WithName(mountCandidate.SourceRepository)
			if err != nil {
				logrus.Errorf("failed to parse source repository reference %v: %v", namedRef.String(), err)
				pd.v2MetadataService.Remove(mountCandidate)
				continue
			}

			// TODO (brianbland): We need to construct a reference where the Name is
			// only the full remote name, so clean this up when distribution has a
			// richer reference package
			remoteRef, err := distreference.WithName(namedRef.RemoteName())
			if err != nil {
				logrus.Errorf("failed to make remote reference out of %q: %v", namedRef.RemoteName(), namedRef.RemoteName())
				continue
			}

			canonicalRef, err := distreference.WithDigest(remoteRef, mountCandidate.Digest)
			if err != nil {
				logrus.Errorf("failed to make canonical reference: %v", err)
				continue
			}

			createOpts = append(createOpts, client.WithMountFrom(canonicalRef))
		}

		// send the layer
		lu, err := bs.Create(ctx, createOpts...)
		switch err := err.(type) {
		case nil:
			// noop
		case distribution.ErrBlobMounted:
			progress.Updatef(progressOutput, pd.ID(), "Mounted from %s", err.From.Name())

			err.Descriptor.MediaType = schema2.MediaTypeLayer

			pd.pushState.Lock()
			pd.pushState.confirmedV2 = true
			pd.pushState.remoteLayers[diffID] = err.Descriptor
			pd.pushState.Unlock()

			// Cache mapping from this layer's DiffID to the blobsum
			if err := pd.v2MetadataService.TagAndAdd(diffID, pd.hmacKey, metadata.V2Metadata{
				Digest:           err.Descriptor.Digest,
				SourceRepository: pd.repoInfo.FullName(),
			}); err != nil {
				return distribution.Descriptor{}, xfer.DoNotRetry{Err: err}
			}
			return err.Descriptor, nil
		default:
			logrus.Infof("failed to mount layer %s (%s) from %s: %v", diffID, mountCandidate.Digest, mountCandidate.SourceRepository, err)
		}

		if len(mountCandidate.SourceRepository) > 0 &&
			(metadata.CheckV2MetadataHMAC(&mountCandidate, pd.hmacKey) ||
				len(mountCandidate.HMAC) == 0) {
			cause := "blob mount failure"
			if err != nil {
				cause = fmt.Sprintf("an error: %v", err.Error())
			}
			logrus.Debugf("removing association between layer %s and %s due to %s", mountCandidate.Digest, mountCandidate.SourceRepository, cause)
			pd.v2MetadataService.Remove(mountCandidate)
		}

		if lu != nil {
			// cancel previous upload
			cancelLayerUpload(ctx, mountCandidate.Digest, layerUpload)
			layerUpload = lu
		}
	}

	if layerUpload == nil {
		layerUpload, err = bs.Create(ctx)
		if err != nil {
			return distribution.Descriptor{}, retryOnError(err)
		}
	}
	defer layerUpload.Close()

	// upload the blob
	desc, err := pd.uploadUsingSession(ctx, progressOutput, diffID, layerUpload)
	if err != nil {
		return desc, err
	}

	pd.pushState.Lock()
	// If Commit succeeded, that's an indication that the remote registry speaks the v2 protocol.
	pd.pushState.confirmedV2 = true
	pd.pushState.remoteLayers[diffID] = desc
	pd.pushState.Unlock()

	return desc, nil
}

func (pd *v2PushDescriptor) SetRemoteDescriptor(descriptor distribution.Descriptor) {
	pd.remoteDescriptor = descriptor
}

func (pd *v2PushDescriptor) Descriptor() distribution.Descriptor {
	return pd.remoteDescriptor
}

func (pd *v2PushDescriptor) uploadUsingSession(
	ctx context.Context,
	progressOutput progress.Output,
	diffID layer.DiffID,
	layerUpload distribution.BlobWriter,
) (distribution.Descriptor, error) {
	arch, err := pd.layer.TarStream()
	if err != nil {
		return distribution.Descriptor{}, xfer.DoNotRetry{Err: err}
	}

	// don't care if this fails; best effort
	size, _ := pd.layer.DiffSize()

	reader := progress.NewProgressReader(ioutils.NewCancelReadCloser(ctx, arch), progressOutput, size, pd.ID(), "Pushing")
	compressedReader, compressionDone := compress(reader)
	defer func() {
		reader.Close()
		<-compressionDone
	}()

	digester := digest.Canonical.New()
	tee := io.TeeReader(compressedReader, digester.Hash())

	nn, err := layerUpload.ReadFrom(tee)
	compressedReader.Close()
	if err != nil {
		return distribution.Descriptor{}, retryOnError(err)
	}

	pushDigest := digester.Digest()
	if _, err := layerUpload.Commit(ctx, distribution.Descriptor{Digest: pushDigest}); err != nil {
		return distribution.Descriptor{}, retryOnError(err)
	}

	logrus.Debugf("uploaded layer %s (%s), %d bytes", diffID, pushDigest, nn)
	progress.Update(progressOutput, pd.ID(), "Pushed")

	// Cache mapping from this layer's DiffID to the blobsum
	if err := pd.v2MetadataService.TagAndAdd(diffID, pd.hmacKey, metadata.V2Metadata{
		Digest:           pushDigest,
		SourceRepository: pd.repoInfo.FullName(),
	}); err != nil {
		return distribution.Descriptor{}, xfer.DoNotRetry{Err: err}
	}

	return distribution.Descriptor{
		Digest:    pushDigest,
		MediaType: schema2.MediaTypeLayer,
		Size:      nn,
	}, nil
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

// getRepositoryMountCandidates returns an array of v2 metadata items belonging to the given registry. The
// array is sorted from youngest to oldest. If requireReigstryMatch is true, the resulting array will contain
// only metadata entries having registry part of SourceRepository matching the part of repoInfo.
func getRepositoryMountCandidates(
	repoInfo reference.Named,
	hmacKey []byte,
	max int,
	v2Metadata []metadata.V2Metadata,
) []metadata.V2Metadata {
	candidates := []metadata.V2Metadata{}
	for _, meta := range v2Metadata {
		sourceRepo, err := reference.ParseNamed(meta.SourceRepository)
		if err != nil || repoInfo.Hostname() != sourceRepo.Hostname() {
			continue
		}
		// target repository is not a viable candidate
		if meta.SourceRepository == repoInfo.FullName() {
			continue
		}
		candidates = append(candidates, meta)
	}

	sortV2MetadataByLikenessAndAge(repoInfo, hmacKey, candidates)
	if max >= 0 && len(candidates) > max {
		// select the youngest metadata
		candidates = candidates[:max]
	}

	return candidates
}

// byLikeness is a sorting container for v2 metadata candidates for cross repository mount. The
// candidate "a" is preferred over "b":
//
//  1. if it was hashed using the same AuthConfig as the one used to authenticate to target repository and the
//     "b" was not
//  2. if a number of its repository path components exactly matching path components of target repository is higher
type byLikeness struct {
	arr            []metadata.V2Metadata
	hmacKey        []byte
	pathComponents []string
}

func (bla byLikeness) Less(i, j int) bool {
	aMacMatch := metadata.CheckV2MetadataHMAC(&bla.arr[i], bla.hmacKey)
	bMacMatch := metadata.CheckV2MetadataHMAC(&bla.arr[j], bla.hmacKey)
	if aMacMatch != bMacMatch {
		return aMacMatch
	}
	aMatch := numOfMatchingPathComponents(bla.arr[i].SourceRepository, bla.pathComponents)
	bMatch := numOfMatchingPathComponents(bla.arr[j].SourceRepository, bla.pathComponents)
	return aMatch > bMatch
}
func (bla byLikeness) Swap(i, j int) {
	bla.arr[i], bla.arr[j] = bla.arr[j], bla.arr[i]
}
func (bla byLikeness) Len() int { return len(bla.arr) }

func sortV2MetadataByLikenessAndAge(repoInfo reference.Named, hmacKey []byte, marr []metadata.V2Metadata) {
	// reverse the metadata array to shift the newest entries to the beginning
	for i := 0; i < len(marr)/2; i++ {
		marr[i], marr[len(marr)-i-1] = marr[len(marr)-i-1], marr[i]
	}
	// keep equal entries ordered from the youngest to the oldest
	sort.Stable(byLikeness{
		arr:            marr,
		hmacKey:        hmacKey,
		pathComponents: getPathComponents(repoInfo.FullName()),
	})
}

// numOfMatchingPathComponents returns a number of path components in "pth" that exactly match "matchComponents".
func numOfMatchingPathComponents(pth string, matchComponents []string) int {
	pthComponents := getPathComponents(pth)
	i := 0
	for ; i < len(pthComponents) && i < len(matchComponents); i++ {
		if matchComponents[i] != pthComponents[i] {
			return i
		}
	}
	return i
}

func getPathComponents(path string) []string {
	// make sure to add docker.io/ prefix to the path
	named, err := reference.ParseNamed(path)
	if err == nil {
		path = named.FullName()
	}
	return strings.Split(path, "/")
}

func cancelLayerUpload(ctx context.Context, dgst digest.Digest, layerUpload distribution.BlobWriter) {
	if layerUpload != nil {
		logrus.Debugf("cancelling upload of blob %s", dgst)
		err := layerUpload.Cancel(ctx)
		if err != nil {
			logrus.Warnf("failed to cancel upload: %v", err)
		}
	}
}
