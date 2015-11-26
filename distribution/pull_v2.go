package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

type v2Puller struct {
	blobSumService *metadata.BlobSumService
	endpoint       registry.APIEndpoint
	config         *ImagePullConfig
	sf             *streamformatter.StreamFormatter
	repoInfo       *registry.RepositoryInfo
	repo           distribution.Repository
	sessionID      string
}

func (p *v2Puller) Pull(ref reference.Named) (fallback bool, err error) {
	// TODO(tiborvass): was ReceiveTimeout
	p.repo, err = NewV2Repository(p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return true, err
	}

	p.sessionID = stringid.GenerateRandomID()

	if err := p.pullV2Repository(ref); err != nil {
		if registry.ContinueOnError(err) {
			logrus.Debugf("Error trying v2 registry: %v", err)
			return true, err
		}
		return false, err
	}
	return false, nil
}

func (p *v2Puller) pullV2Repository(ref reference.Named) (err error) {
	var refs []reference.Named
	taggedName := p.repoInfo.LocalName
	if tagged, isTagged := ref.(reference.Tagged); isTagged {
		taggedName, err = reference.WithTag(p.repoInfo.LocalName, tagged.Tag())
		if err != nil {
			return err
		}
		refs = []reference.Named{taggedName}
	} else if digested, isDigested := ref.(reference.Digested); isDigested {
		taggedName, err = reference.WithDigest(p.repoInfo.LocalName, digested.Digest())
		if err != nil {
			return err
		}
		refs = []reference.Named{taggedName}
	} else {
		manSvc, err := p.repo.Manifests(context.Background())
		if err != nil {
			return err
		}

		tags, err := manSvc.Tags()
		if err != nil {
			return err
		}

		// This probably becomes a lot nicer after the manifest
		// refactor...
		for _, tag := range tags {
			tagRef, err := reference.WithTag(p.repoInfo.LocalName, tag)
			if err != nil {
				return err
			}
			refs = append(refs, tagRef)
		}
	}

	var layersDownloaded bool
	for _, pullRef := range refs {
		// pulledNew is true if either new layers were downloaded OR if existing images were newly tagged
		// TODO(tiborvass): should we change the name of `layersDownload`? What about message in WriteStatus?
		pulledNew, err := p.pullV2Tag(p.config.OutStream, pullRef)
		if err != nil {
			return err
		}
		layersDownloaded = layersDownloaded || pulledNew
	}

	writeStatus(taggedName.String(), p.config.OutStream, p.sf, layersDownloaded)

	return nil
}

// downloadInfo is used to pass information from download to extractor
type downloadInfo struct {
	tmpFile     *os.File
	digest      digest.Digest
	layer       distribution.ReadSeekCloser
	size        int64
	err         chan error
	poolKey     string
	broadcaster *broadcaster.Buffered
}

type errVerification struct{}

func (errVerification) Error() string { return "verification failed" }

func (p *v2Puller) download(di *downloadInfo) {
	logrus.Debugf("pulling blob %q", di.digest)

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

	digestStr := di.digest.String()

	reader := progressreader.New(progressreader.Config{
		In:        ioutil.NopCloser(io.TeeReader(layerDownload, verifier)),
		Out:       di.broadcaster,
		Formatter: p.sf,
		Size:      di.size,
		NewLines:  false,
		ID:        stringid.TruncateID(digestStr),
		Action:    "Downloading",
	})
	io.Copy(di.tmpFile, reader)

	di.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(digestStr), "Verifying Checksum", nil))

	if !verifier.Verified() {
		err = fmt.Errorf("filesystem layer verification failed for digest %s", di.digest)
		logrus.Error(err)
		di.err <- err
		return
	}

	di.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(digestStr), "Download complete", nil))

	logrus.Debugf("Downloaded %s to tempfile %s", digestStr, di.tmpFile.Name())
	di.layer = layerDownload

	di.err <- nil
}

func (p *v2Puller) pullV2Tag(out io.Writer, ref reference.Named) (tagUpdated bool, err error) {
	tagOrDigest := ""
	if tagged, isTagged := ref.(reference.Tagged); isTagged {
		tagOrDigest = tagged.Tag()
	} else if digested, isDigested := ref.(reference.Digested); isDigested {
		tagOrDigest = digested.Digest().String()
	} else {
		return false, fmt.Errorf("internal error: reference has neither a tag nor a digest: %s", ref.String())
	}

	logrus.Debugf("Pulling ref from V2 registry: %q", tagOrDigest)

	manSvc, err := p.repo.Manifests(context.Background())
	if err != nil {
		return false, err
	}

	unverifiedManifest, err := manSvc.GetByTag(tagOrDigest)
	if err != nil {
		return false, err
	}
	if unverifiedManifest == nil {
		return false, fmt.Errorf("image manifest does not exist for tag or digest %q", tagOrDigest)
	}
	var verifiedManifest *schema1.Manifest
	verifiedManifest, err = verifyManifest(unverifiedManifest, ref)
	if err != nil {
		return false, err
	}

	rootFS := image.NewRootFS()

	if err := detectBaseLayer(p.config.ImageStore, verifiedManifest, rootFS); err != nil {
		return false, err
	}

	// remove duplicate layers and check parent chain validity
	err = fixManifestLayers(verifiedManifest)
	if err != nil {
		return false, err
	}

	out.Write(p.sf.FormatStatus(tagOrDigest, "Pulling from %s", p.repo.Name()))

	var downloads []*downloadInfo

	defer func() {
		for _, d := range downloads {
			p.config.Pool.removeWithError(d.poolKey, err)
			if d.tmpFile != nil {
				d.tmpFile.Close()
				if err := os.RemoveAll(d.tmpFile.Name()); err != nil {
					logrus.Errorf("Failed to remove temp file: %s", d.tmpFile.Name())
				}
			}
		}
	}()

	// Image history converted to the new format
	var history []image.History

	poolKey := "v2layer:"
	notFoundLocally := false

	// Note that the order of this loop is in the direction of bottom-most
	// to top-most, so that the downloads slice gets ordered correctly.
	for i := len(verifiedManifest.FSLayers) - 1; i >= 0; i-- {
		blobSum := verifiedManifest.FSLayers[i].BlobSum
		poolKey += blobSum.String()

		var throwAway struct {
			ThrowAway bool `json:"throwaway,omitempty"`
		}
		if err := json.Unmarshal([]byte(verifiedManifest.History[i].V1Compatibility), &throwAway); err != nil {
			return false, err
		}

		h, err := v1.HistoryFromConfig([]byte(verifiedManifest.History[i].V1Compatibility), throwAway.ThrowAway)
		if err != nil {
			return false, err
		}
		history = append(history, h)

		if throwAway.ThrowAway {
			continue
		}

		// Do we have a layer on disk corresponding to the set of
		// blobsums up to this point?
		if !notFoundLocally {
			notFoundLocally = true
			diffID, err := p.blobSumService.GetDiffID(blobSum)
			if err == nil {
				rootFS.Append(diffID)
				if l, err := p.config.LayerStore.Get(rootFS.ChainID()); err == nil {
					notFoundLocally = false
					logrus.Debugf("Layer already exists: %s", blobSum.String())
					out.Write(p.sf.FormatProgress(stringid.TruncateID(blobSum.String()), "Already exists", nil))
					defer layer.ReleaseAndLog(p.config.LayerStore, l)
					continue
				} else {
					rootFS.DiffIDs = rootFS.DiffIDs[:len(rootFS.DiffIDs)-1]
				}
			}
		}

		out.Write(p.sf.FormatProgress(stringid.TruncateID(blobSum.String()), "Pulling fs layer", nil))

		tmpFile, err := ioutil.TempFile("", "GetImageBlob")
		if err != nil {
			return false, err
		}

		d := &downloadInfo{
			poolKey: poolKey,
			digest:  blobSum,
			tmpFile: tmpFile,
			// TODO: seems like this chan buffer solved hanging problem in go1.5,
			// this can indicate some deeper problem that somehow we never take
			// error from channel in loop below
			err: make(chan error, 1),
		}

		downloads = append(downloads, d)

		broadcaster, found := p.config.Pool.add(d.poolKey)
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

			diffID, err := p.blobSumService.GetDiffID(d.digest)
			if err != nil {
				return false, err
			}
			rootFS.Append(diffID)

			l, err := p.config.LayerStore.Get(rootFS.ChainID())
			if err != nil {
				return false, err
			}

			defer layer.ReleaseAndLog(p.config.LayerStore, l)

			continue
		}

		d.tmpFile.Seek(0, 0)
		reader := progressreader.New(progressreader.Config{
			In:        d.tmpFile,
			Out:       d.broadcaster,
			Formatter: p.sf,
			Size:      d.size,
			NewLines:  false,
			ID:        stringid.TruncateID(d.digest.String()),
			Action:    "Extracting",
		})

		inflatedLayerData, err := archive.DecompressStream(reader)
		if err != nil {
			return false, fmt.Errorf("could not get decompression stream: %v", err)
		}

		l, err := p.config.LayerStore.Register(inflatedLayerData, rootFS.ChainID())
		if err != nil {
			return false, fmt.Errorf("failed to register layer: %v", err)
		}
		logrus.Debugf("layer %s registered successfully", l.DiffID())
		rootFS.Append(l.DiffID())

		// Cache mapping from this layer's DiffID to the blobsum
		if err := p.blobSumService.Add(l.DiffID(), d.digest); err != nil {
			return false, err
		}

		defer layer.ReleaseAndLog(p.config.LayerStore, l)

		d.broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(d.digest.String()), "Pull complete", nil))
		d.broadcaster.Close()
		tagUpdated = true
	}

	config, err := v1.MakeConfigFromV1Config([]byte(verifiedManifest.History[0].V1Compatibility), rootFS, history)
	if err != nil {
		return false, err
	}

	imageID, err := p.config.ImageStore.Create(config)
	if err != nil {
		return false, err
	}

	manifestDigest, _, err := digestFromManifest(unverifiedManifest, p.repoInfo.LocalName.Name())
	if err != nil {
		return false, err
	}

	// Check for new tag if no layers downloaded
	var oldTagImageID image.ID
	if !tagUpdated {
		oldTagImageID, err = p.config.TagStore.Get(ref)
		if err != nil || oldTagImageID != imageID {
			tagUpdated = true
		}
	}

	if tagUpdated {
		if canonical, ok := ref.(reference.Canonical); ok {
			if err = p.config.TagStore.AddDigest(canonical, imageID, true); err != nil {
				return false, err
			}
		} else if err = p.config.TagStore.AddTag(ref, imageID, true); err != nil {
			return false, err
		}
	}

	if manifestDigest != "" {
		out.Write(p.sf.FormatStatus("", "Digest: %s", manifestDigest))
	}

	return tagUpdated, nil
}

func verifyManifest(signedManifest *schema1.SignedManifest, ref reference.Reference) (m *schema1.Manifest, err error) {
	// If pull by digest, then verify the manifest digest. NOTE: It is
	// important to do this first, before any other content validation. If the
	// digest cannot be verified, don't even bother with those other things.
	if digested, isDigested := ref.(reference.Digested); isDigested {
		verifier, err := digest.NewDigestVerifier(digested.Digest())
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
			err := fmt.Errorf("image verification failed for digest %s", digested.Digest())
			logrus.Error(err)
			return nil, err
		}

		var verifiedManifest schema1.Manifest
		if err = json.Unmarshal(payload, &verifiedManifest); err != nil {
			return nil, err
		}
		m = &verifiedManifest
	} else {
		m = &signedManifest.Manifest
	}

	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema version %d for %q", m.SchemaVersion, ref.String())
	}
	if len(m.FSLayers) != len(m.History) {
		return nil, fmt.Errorf("length of history not equal to number of layers for %q", ref.String())
	}
	if len(m.FSLayers) == 0 {
		return nil, fmt.Errorf("no FSLayers in manifest for %q", ref.String())
	}
	return m, nil
}

// fixManifestLayers removes repeated layers from the manifest and checks the
// correctness of the parent chain.
func fixManifestLayers(m *schema1.Manifest) error {
	imgs := make([]*image.V1Image, len(m.FSLayers))
	for i := range m.FSLayers {
		img := &image.V1Image{}

		if err := json.Unmarshal([]byte(m.History[i].V1Compatibility), img); err != nil {
			return err
		}

		imgs[i] = img
		if err := v1.ValidateID(img.ID); err != nil {
			return err
		}
	}

	if imgs[len(imgs)-1].Parent != "" && runtime.GOOS != "windows" {
		// Windows base layer can point to a base layer parent that is not in manifest.
		return errors.New("Invalid parent ID in the base layer of the image.")
	}

	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})

	var lastID string
	for _, img := range imgs {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return fmt.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}

	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(imgs) - 2; i >= 0; i-- {
		if imgs[i].ID == imgs[i+1].ID { // repeated ID. remove and continue
			m.FSLayers = append(m.FSLayers[:i], m.FSLayers[i+1:]...)
			m.History = append(m.History[:i], m.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return fmt.Errorf("Invalid parent ID. Expected %v, got %v.", imgs[i+1].ID, imgs[i].Parent)
		}
	}

	return nil
}
