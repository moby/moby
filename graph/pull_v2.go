package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"golang.org/x/net/context"
)

type v2Puller struct {
	*TagStore
	endpoint  registry.APIEndpoint
	config    *ImagePullConfig
	sf        *streamformatter.StreamFormatter
	repoInfo  *registry.RepositoryInfo
	repo      distribution.Repository
	sessionID string
}

func (p *v2Puller) Pull(tag string) (fallback bool, err error) {
	// TODO(tiborvass): was ReceiveTimeout
	p.repo, err = NewV2Repository(p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	if err != nil {
		logrus.Warnf("Error getting v2 registry: %v", err)
		return true, err
	}

	p.sessionID = stringid.GenerateRandomID()

	if err := p.pullV2Repository(tag); err != nil {
		if registry.ContinueOnError(err) {
			logrus.Debugf("Error trying v2 registry: %v", err)
			return true, err
		}
		return false, err
	}
	return false, nil
}

func (p *v2Puller) pullV2Repository(tag string) (err error) {
	var tags []string
	taggedName := p.repoInfo.LocalName
	if len(tag) > 0 {
		tags = []string{tag}
		taggedName = utils.ImageReference(p.repoInfo.LocalName, tag)
	} else {
		var err error

		manSvc, err := p.repo.Manifests(context.Background())
		if err != nil {
			return err
		}

		tags, err = manSvc.Tags()
		if err != nil {
			return err
		}

	}

	poolKey := "v2:" + taggedName
	broadcaster, found := p.poolAdd("pull", poolKey)
	broadcaster.Add(p.config.OutStream)
	if found {
		// Another pull of the same repository is already taking place; just wait for it to finish
		return broadcaster.Wait()
	}

	// This must use a closure so it captures the value of err when the
	// function returns, not when the 'defer' is evaluated.
	defer func() {
		p.poolRemoveWithError("pull", poolKey, err)
	}()

	var layersDownloaded bool
	for _, tag := range tags {
		// pulledNew is true if either new layers were downloaded OR if existing images were newly tagged
		// TODO(tiborvass): should we change the name of `layersDownload`? What about message in WriteStatus?
		pulledNew, err := p.pullV2Tag(broadcaster, tag, taggedName)
		if err != nil {
			return err
		}
		layersDownloaded = layersDownloaded || pulledNew
	}

	writeStatus(taggedName, broadcaster, p.sf, layersDownloaded)

	return nil
}

// downloadInfo is used to pass information from download to extractor
type downloadInfo struct {
	img         contentAddressableDescriptor
	imgIndex    int
	tmpFile     *os.File
	digest      digest.Digest
	layer       distribution.ReadSeekCloser
	size        int64
	err         chan error
	poolKey     string
	broadcaster *broadcaster.Buffered
}

// contentAddressableDescriptor is used to pass image data from a manifest to the
// graph.
type contentAddressableDescriptor struct {
	id              string
	parent          string
	strongID        digest.Digest
	compatibilityID string
	config          []byte
	v1Compatibility []byte
}

func newContentAddressableImage(v1Compatibility []byte, blobSum digest.Digest, parent digest.Digest) (contentAddressableDescriptor, error) {
	img := contentAddressableDescriptor{
		v1Compatibility: v1Compatibility,
	}

	var err error
	img.config, err = image.MakeImageConfig(v1Compatibility, blobSum, parent)
	if err != nil {
		return img, err
	}
	img.strongID, err = image.StrongID(img.config)
	if err != nil {
		return img, err
	}

	unmarshalledConfig, err := image.NewImgJSON(v1Compatibility)
	if err != nil {
		return img, err
	}

	img.compatibilityID = unmarshalledConfig.ID
	img.id = img.strongID.Hex()

	return img, nil
}

// ID returns the actual ID to be used for the downloaded image. This may be
// a computed ID.
func (img contentAddressableDescriptor) ID() string {
	return img.id
}

// Parent returns the parent ID to be used for the image. This may be a
// computed ID.
func (img contentAddressableDescriptor) Parent() string {
	return img.parent
}

// MarshalConfig renders the image structure into JSON.
func (img contentAddressableDescriptor) MarshalConfig() ([]byte, error) {
	return img.config, nil
}

type errVerification struct{}

func (errVerification) Error() string { return "verification failed" }

func (p *v2Puller) download(di *downloadInfo) {
	logrus.Debugf("pulling blob %q to %s", di.digest, di.img.id)

	blobs := p.repo.Blobs(context.Background())

	desc, err := blobs.Stat(context.Background(), di.digest)
	if err != nil {
		logrus.Debugf("Error statting layer: %v", err)
		di.err <- err
		return
	}
	di.size = desc.Size

	layerDownload, err := blobs.Open(context.Background(), di.digest)
	if err != nil {
		logrus.Debugf("Error fetching layer: %v", err)
		di.err <- err
		return
	}
	defer layerDownload.Close()

	verifier, err := digest.NewDigestVerifier(di.digest)
	if err != nil {
		di.err <- err
		return
	}

	reader := progressreader.New(progressreader.Config{
		In:        ioutil.NopCloser(io.TeeReader(layerDownload, verifier)),
		Out:       di.broadcaster,
		Formatter: p.sf,
		Size:      di.size,
		NewLines:  false,
		ID:        stringid.TruncateID(di.img.id),
		Action:    "Downloading",
	})
	io.Copy(di.tmpFile, reader)

	di.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(di.img.id), "Verifying Checksum", nil))

	if !verifier.Verified() {
		err = fmt.Errorf("filesystem layer verification failed for digest %s", di.digest)
		logrus.Error(err)
		di.err <- err
		return
	}

	di.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(di.img.id), "Download complete", nil))

	logrus.Debugf("Downloaded %s to tempfile %s", di.img.id, di.tmpFile.Name())
	di.layer = layerDownload

	di.err <- nil
}

func (p *v2Puller) pullV2Tag(out io.Writer, tag, taggedName string) (tagUpdated bool, err error) {
	logrus.Debugf("Pulling tag from V2 registry: %q", tag)

	manSvc, err := p.repo.Manifests(context.Background())
	if err != nil {
		return false, err
	}

	unverifiedManifest, err := manSvc.GetByTag(tag)
	if err != nil {
		return false, err
	}
	if unverifiedManifest == nil {
		return false, fmt.Errorf("image manifest does not exist for tag %q", tag)
	}
	var verifiedManifest *manifest.Manifest
	verifiedManifest, err = verifyManifest(unverifiedManifest, tag)
	if err != nil {
		return false, err
	}

	// remove duplicate layers and check parent chain validity
	err = fixManifestLayers(verifiedManifest)
	if err != nil {
		return false, err
	}

	imgs, err := p.getImageInfos(verifiedManifest)
	if err != nil {
		return false, err
	}

	out.Write(p.sf.FormatStatus(tag, "Pulling from %s", p.repo.Name()))

	var downloads []*downloadInfo

	var layerIDs []string
	defer func() {
		p.graph.Release(p.sessionID, layerIDs...)

		for _, d := range downloads {
			p.poolRemoveWithError("pull", d.poolKey, err)
			if d.tmpFile != nil {
				d.tmpFile.Close()
				if err := os.RemoveAll(d.tmpFile.Name()); err != nil {
					logrus.Errorf("Failed to remove temp file: %s", d.tmpFile.Name())
				}
			}
		}
	}()

	for i := len(verifiedManifest.FSLayers) - 1; i >= 0; i-- {
		img := imgs[i]

		p.graph.Retain(p.sessionID, img.id)
		layerIDs = append(layerIDs, img.id)

		p.graph.imageMutex.Lock(img.id)

		// Check if exists
		if p.graph.Exists(img.id) {
			if err := p.validateImageInGraph(img.id, imgs, i); err != nil {
				p.graph.imageMutex.Unlock(img.id)
				return false, fmt.Errorf("image validation failed: %v", err)
			}
			logrus.Debugf("Image already exists: %s", img.id)
			p.graph.imageMutex.Unlock(img.id)
			continue
		}
		p.graph.imageMutex.Unlock(img.id)

		out.Write(p.sf.FormatProgress(stringid.TruncateID(img.id), "Pulling fs layer", nil))

		d := &downloadInfo{
			img:      img,
			imgIndex: i,
			poolKey:  "v2layer:" + img.id,
			digest:   verifiedManifest.FSLayers[i].BlobSum,
			// TODO: seems like this chan buffer solved hanging problem in go1.5,
			// this can indicate some deeper problem that somehow we never take
			// error from channel in loop below
			err: make(chan error, 1),
		}

		tmpFile, err := ioutil.TempFile("", "GetImageBlob")
		if err != nil {
			return false, err
		}
		d.tmpFile = tmpFile

		downloads = append(downloads, d)

		broadcaster, found := p.poolAdd("pull", d.poolKey)
		broadcaster.Add(out)
		d.broadcaster = broadcaster
		if found {
			d.err <- nil
		} else {
			go p.download(d)
		}
	}

	for _, d := range downloads {
		if err := <-d.err; err != nil {
			return false, err
		}

		if d.layer == nil {
			// Wait for a different pull to download and extract
			// this layer.
			err = d.broadcaster.Wait()
			if err != nil {
				return false, err
			}
			continue
		}

		d.tmpFile.Seek(0, 0)
		err := func() error {
			reader := progressreader.New(progressreader.Config{
				In:        d.tmpFile,
				Out:       d.broadcaster,
				Formatter: p.sf,
				Size:      d.size,
				NewLines:  false,
				ID:        stringid.TruncateID(d.img.id),
				Action:    "Extracting",
			})

			p.graph.imagesMutex.Lock()
			defer p.graph.imagesMutex.Unlock()

			p.graph.imageMutex.Lock(d.img.id)
			defer p.graph.imageMutex.Unlock(d.img.id)

			// Must recheck the data on disk if any exists.
			// This protects against races where something
			// else is written to the graph under this ID
			// after attemptIDReuse.
			if p.graph.Exists(d.img.id) {
				if err := p.validateImageInGraph(d.img.id, imgs, d.imgIndex); err != nil {
					return fmt.Errorf("image validation failed: %v", err)
				}
			}

			if err := p.graph.register(d.img, reader); err != nil {
				return err
			}

			if err := p.graph.setLayerDigest(d.img.id, d.digest); err != nil {
				return err
			}

			if err := p.graph.setV1CompatibilityConfig(d.img.id, d.img.v1Compatibility); err != nil {
				return err
			}

			return nil
		}()
		if err != nil {
			return false, err
		}

		d.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(d.img.id), "Pull complete", nil))
		d.broadcaster.Close()
		tagUpdated = true
	}

	manifestDigest, _, err := digestFromManifest(unverifiedManifest, p.repoInfo.LocalName)
	if err != nil {
		return false, err
	}

	// Check for new tag if no layers downloaded
	if !tagUpdated {
		repo, err := p.Get(p.repoInfo.LocalName)
		if err != nil {
			return false, err
		}
		if repo != nil {
			if _, exists := repo[tag]; !exists {
				tagUpdated = true
			}
		} else {
			tagUpdated = true
		}
	}

	firstID := layerIDs[len(layerIDs)-1]
	if utils.DigestReference(tag) {
		// TODO(stevvooe): Ideally, we should always set the digest so we can
		// use the digest whether we pull by it or not. Unfortunately, the tag
		// store treats the digest as a separate tag, meaning there may be an
		// untagged digest image that would seem to be dangling by a user.
		if err = p.SetDigest(p.repoInfo.LocalName, tag, firstID); err != nil {
			return false, err
		}
	} else {
		// only set the repository/tag -> image ID mapping when pulling by tag (i.e. not by digest)
		if err = p.Tag(p.repoInfo.LocalName, tag, firstID, true); err != nil {
			return false, err
		}
	}

	if manifestDigest != "" {
		out.Write(p.sf.FormatStatus("", "Digest: %s", manifestDigest))
	}

	return tagUpdated, nil
}

func verifyManifest(signedManifest *manifest.SignedManifest, tag string) (m *manifest.Manifest, err error) {
	// If pull by digest, then verify the manifest digest. NOTE: It is
	// important to do this first, before any other content validation. If the
	// digest cannot be verified, don't even bother with those other things.
	if manifestDigest, err := digest.ParseDigest(tag); err == nil {
		verifier, err := digest.NewDigestVerifier(manifestDigest)
		if err != nil {
			return nil, err
		}
		payload, err := signedManifest.Payload()
		if err != nil {
			// If this failed, the signatures section was corrupted
			// or missing. Treat the entire manifest as the payload.
			payload = signedManifest.Raw
		}
		if _, err := verifier.Write(payload); err != nil {
			return nil, err
		}
		if !verifier.Verified() {
			err := fmt.Errorf("image verification failed for digest %s", manifestDigest)
			logrus.Error(err)
			return nil, err
		}

		var verifiedManifest manifest.Manifest
		if err = json.Unmarshal(payload, &verifiedManifest); err != nil {
			return nil, err
		}
		m = &verifiedManifest
	} else {
		m = &signedManifest.Manifest
	}

	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema version %d for tag %q", m.SchemaVersion, tag)
	}
	if len(m.FSLayers) != len(m.History) {
		return nil, fmt.Errorf("length of history not equal to number of layers for tag %q", tag)
	}
	if len(m.FSLayers) == 0 {
		return nil, fmt.Errorf("no FSLayers in manifest for tag %q", tag)
	}
	return m, nil
}

// fixManifestLayers removes repeated layers from the manifest and checks the
// correctness of the parent chain.
func fixManifestLayers(m *manifest.Manifest) error {
	images := make([]*image.Image, len(m.FSLayers))
	for i := range m.FSLayers {
		img, err := image.NewImgJSON([]byte(m.History[i].V1Compatibility))
		if err != nil {
			return err
		}
		images[i] = img
		if err := image.ValidateID(img.ID); err != nil {
			return err
		}
	}

	if images[len(images)-1].Parent != "" {
		return errors.New("Invalid parent ID in the base layer of the image.")
	}

	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})

	var lastID string
	for _, img := range images {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return fmt.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}

	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(images) - 2; i >= 0; i-- {
		if images[i].ID == images[i+1].ID { // repeated ID. remove and continue
			m.FSLayers = append(m.FSLayers[:i], m.FSLayers[i+1:]...)
			m.History = append(m.History[:i], m.History[i+1:]...)
		} else if images[i].Parent != images[i+1].ID {
			return fmt.Errorf("Invalid parent ID. Expected %v, got %v.", images[i+1].ID, images[i].Parent)
		}
	}

	return nil
}

// getImageInfos returns an imageinfo struct for every image in the manifest.
// These objects contain both calculated strongIDs and compatibilityIDs found
// in v1Compatibility object.
func (p *v2Puller) getImageInfos(m *manifest.Manifest) ([]contentAddressableDescriptor, error) {
	imgs := make([]contentAddressableDescriptor, len(m.FSLayers))

	var parent digest.Digest
	for i := len(imgs) - 1; i >= 0; i-- {
		var err error
		imgs[i], err = newContentAddressableImage([]byte(m.History[i].V1Compatibility), m.FSLayers[i].BlobSum, parent)
		if err != nil {
			return nil, err
		}
		parent = imgs[i].strongID
	}

	p.attemptIDReuse(imgs)

	return imgs, nil
}

// attemptIDReuse does a best attempt to match verified compatibilityIDs
// already in the graph with the computed strongIDs so we can keep using them.
// This process will never fail but may just return the strongIDs if none of
// the compatibilityIDs exists or can be verified. If the strongIDs themselves
// fail verification, we deterministically generate alternate IDs to use until
// we find one that's available or already exists with the correct data.
func (p *v2Puller) attemptIDReuse(imgs []contentAddressableDescriptor) {
	// This function needs to be protected with a global lock, because it
	// locks multiple IDs at once, and there's no good way to make sure
	// the locking happens a deterministic order.
	p.graph.imagesMutex.Lock()
	defer p.graph.imagesMutex.Unlock()

	idMap := make(map[string]struct{})
	for _, img := range imgs {
		idMap[img.id] = struct{}{}
		idMap[img.compatibilityID] = struct{}{}

		if p.graph.Exists(img.compatibilityID) {
			if _, err := p.graph.GenerateV1CompatibilityChain(img.compatibilityID); err != nil {
				logrus.Debugf("Migration v1Compatibility generation error: %v", err)
				return
			}
		}
	}
	for id := range idMap {
		p.graph.imageMutex.Lock(id)
		defer p.graph.imageMutex.Unlock(id)
	}

	// continueReuse controls whether the function will try to find
	// existing layers on disk under the old v1 IDs, to avoid repulling
	// them. The hashes are checked to ensure these layers are okay to
	// use. continueReuse starts out as true, but is set to false if
	// the code encounters something that doesn't match the expected hash.
	continueReuse := true

	for i := len(imgs) - 1; i >= 0; i-- {
		if p.graph.Exists(imgs[i].id) {
			// Found an image in the graph under the strongID. Validate the
			// image before using it.
			if err := p.validateImageInGraph(imgs[i].id, imgs, i); err != nil {
				continueReuse = false
				logrus.Debugf("not using existing strongID: %v", err)

				// The strong ID existed in the graph but didn't
				// validate successfully. We can't use the strong ID
				// because it didn't validate successfully. Treat the
				// graph like a hash table with probing... compute
				// SHA256(id) until we find an ID that either doesn't
				// already exist in the graph, or has existing content
				// that validates successfully.
				for {
					if err := p.tryNextID(imgs, i, idMap); err != nil {
						logrus.Debug(err.Error())
					} else {
						break
					}
				}
			}
			continue
		}

		if continueReuse {
			compatibilityID := imgs[i].compatibilityID
			if err := p.validateImageInGraph(compatibilityID, imgs, i); err != nil {
				logrus.Debugf("stopping ID reuse: %v", err)
				continueReuse = false
			} else {
				// The compatibility ID exists in the graph and was
				// validated. Use it.
				imgs[i].id = compatibilityID
			}
		}
	}

	// fix up the parents of the images
	for i := 0; i < len(imgs); i++ {
		if i == len(imgs)-1 { // Base layer
			imgs[i].parent = ""
		} else {
			imgs[i].parent = imgs[i+1].id
		}
	}
}

// validateImageInGraph checks that an image in the graph has the expected
// strongID. id is the entry in the graph to check, imgs is the slice of
// images being processed (for access to the parent), and i is the index
// into this slice which the graph entry should be checked against.
func (p *v2Puller) validateImageInGraph(id string, imgs []contentAddressableDescriptor, i int) error {
	img, err := p.graph.Get(id)
	if err != nil {
		return fmt.Errorf("missing: %v", err)
	}
	layerID, err := p.graph.getLayerDigest(id)
	if err != nil {
		return fmt.Errorf("digest: %v", err)
	}
	var parentID digest.Digest
	if i != len(imgs)-1 {
		if img.Parent != imgs[i+1].id { // comparing that graph points to validated ID
			return fmt.Errorf("parent: %v %v", img.Parent, imgs[i+1].id)
		}
		parentID = imgs[i+1].strongID
	} else if img.Parent != "" {
		return fmt.Errorf("unexpected parent: %v", img.Parent)
	}

	v1Config, err := p.graph.getV1CompatibilityConfig(img.ID)
	if err != nil {
		return fmt.Errorf("v1Compatibility: %v %v", img.ID, err)
	}

	json, err := image.MakeImageConfig(v1Config, layerID, parentID)
	if err != nil {
		return fmt.Errorf("make config: %v", err)
	}

	if dgst, err := image.StrongID(json); err == nil && dgst == imgs[i].strongID {
		logrus.Debugf("Validated %v as %v", dgst, id)
	} else {
		return fmt.Errorf("digest mismatch: %v %v, error: %v", dgst, imgs[i].strongID, err)
	}

	// All clear
	return nil
}

func (p *v2Puller) tryNextID(imgs []contentAddressableDescriptor, i int, idMap map[string]struct{}) error {
	nextID, _ := digest.FromBytes([]byte(imgs[i].id))
	imgs[i].id = nextID.Hex()

	if _, exists := idMap[imgs[i].id]; !exists {
		p.graph.imageMutex.Lock(imgs[i].id)
		defer p.graph.imageMutex.Unlock(imgs[i].id)
	}

	if p.graph.Exists(imgs[i].id) {
		if err := p.validateImageInGraph(imgs[i].id, imgs, i); err != nil {
			return fmt.Errorf("not using existing strongID permutation %s: %v", imgs[i].id, err)
		}
	}
	return nil
}
