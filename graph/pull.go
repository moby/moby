package graph

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

type ImagePullConfig struct {
	MetaHeaders map[string][]string
	AuthConfig  *cliconfig.AuthConfig
	OutStream   io.Writer
}

func (s *TagStore) Pull(image string, tag string, imagePullConfig *ImagePullConfig) error {
	var (
		sf = streamformatter.NewJSONStreamFormatter()
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(image)
	if err != nil {
		return err
	}

	if err := validateRepoName(repoInfo.LocalName); err != nil {
		return err
	}

	c, err := s.poolAdd("pull", utils.ImageReference(repoInfo.LocalName, tag))
	if err != nil {
		if c != nil {
			// Another pull of the same repository is already taking place; just wait for it to finish
			imagePullConfig.OutStream.Write(sf.FormatStatus("", "Repository %s already being pulled by another client. Waiting.", repoInfo.LocalName))
			<-c
			return nil
		}
		return err
	}
	defer s.poolRemove("pull", utils.ImageReference(repoInfo.LocalName, tag))

	logrus.Debugf("pulling image from host %q with remote name %q", repoInfo.Index.Name, repoInfo.RemoteName)
	endpoint, err := repoInfo.GetEndpoint()
	if err != nil {
		return err
	}

	r, err := registry.NewSession(imagePullConfig.AuthConfig, registry.HTTPRequestFactory(imagePullConfig.MetaHeaders), endpoint, true)
	if err != nil {
		return err
	}

	logName := repoInfo.LocalName
	if tag != "" {
		logName = utils.ImageReference(logName, tag)
	}

	if len(repoInfo.Index.Mirrors) == 0 && (repoInfo.Index.Official || endpoint.Version == registry.APIVersion2) {
		if repoInfo.Official {
			s.trustService.UpdateBase()
		}

		logrus.Debugf("pulling v2 repository with local name %q", repoInfo.LocalName)
		if err := s.pullV2Repository(r, imagePullConfig.OutStream, repoInfo, tag, sf); err == nil {
			s.eventsService.Log("pull", logName, "")
			return nil
		} else if err != registry.ErrDoesNotExist && err != ErrV2RegistryUnavailable {
			logrus.Errorf("Error from V2 registry: %s", err)
		}

		logrus.Debug("image does not exist on v2 registry, falling back to v1")
	}

	logrus.Debugf("pulling v1 repository with local name %q", repoInfo.LocalName)
	if err = s.pullRepository(r, imagePullConfig.OutStream, repoInfo, tag, sf); err != nil {
		return err
	}

	s.eventsService.Log("pull", logName, "")

	return nil
}

func (s *TagStore) pullRepository(r *registry.Session, out io.Writer, repoInfo *registry.RepositoryInfo, askedTag string, sf *streamformatter.StreamFormatter) error {
	out.Write(sf.FormatStatus("", "Pulling repository %s", repoInfo.CanonicalName))

	repoData, err := r.GetRepositoryData(repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return fmt.Errorf("Error: image %s not found", utils.ImageReference(repoInfo.RemoteName, askedTag))
		}
		// Unexpected HTTP error
		return err
	}

	logrus.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, repoInfo.RemoteName, repoData.Tokens)
	if err != nil {
		logrus.Errorf("unable to get remote tags: %s", err)
		return err
	}

	for tag, id := range tagsList {
		repoData.ImgList[id] = &registry.ImgData{
			ID:       id,
			Tag:      tag,
			Checksum: "",
		}
	}

	logrus.Debugf("Registering tags")
	// If no tag has been specified, pull them all
	if askedTag == "" {
		for tag, id := range tagsList {
			repoData.ImgList[id].Tag = tag
		}
	} else {
		// Otherwise, check that the tag exists and use only that one
		id, exists := tagsList[askedTag]
		if !exists {
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, repoInfo.CanonicalName)
		}
		repoData.ImgList[id].Tag = askedTag
	}

	errors := make(chan error)

	layersDownloaded := false
	for _, image := range repoData.ImgList {
		downloadImage := func(img *registry.ImgData) {
			if askedTag != "" && img.Tag != askedTag {
				errors <- nil
				return
			}

			if img.Tag == "" {
				logrus.Debugf("Image (id: %s) present in this repository but untagged, skipping", img.ID)
				errors <- nil
				return
			}

			// ensure no two downloads of the same image happen at the same time
			if c, err := s.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))
				} else {
					logrus.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
				errors <- nil
				return
			}
			defer s.poolRemove("pull", "img:"+img.ID)

			out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, repoInfo.CanonicalName), nil))
			success := false
			var lastErr, err error
			var isDownloaded bool
			for _, ep := range repoInfo.Index.Mirrors {
				out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, mirror: %s", img.Tag, repoInfo.CanonicalName, ep), nil))
				if isDownloaded, err = s.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
					// Don't report errors when pulling from mirrors.
					logrus.Debugf("Error pulling image (%s) from %s, mirror: %s, %s", img.Tag, repoInfo.CanonicalName, ep, err)
					continue
				}
				layersDownloaded = layersDownloaded || isDownloaded
				success = true
				break
			}
			if !success {
				for _, ep := range repoData.Endpoints {
					out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, repoInfo.CanonicalName, ep), nil))
					if isDownloaded, err = s.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
						// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
						// As the error is also given to the output stream the user will see the error.
						lastErr = err
						out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, endpoint: %s, %s", img.Tag, repoInfo.CanonicalName, ep, err), nil))
						continue
					}
					layersDownloaded = layersDownloaded || isDownloaded
					success = true
					break
				}
			}
			if !success {
				err := fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, repoInfo.CanonicalName, lastErr)
				out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), err.Error(), nil))
				errors <- err
				return
			}
			out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))

			errors <- nil
		}

		go downloadImage(image)
	}

	var lastError error
	for i := 0; i < len(repoData.ImgList); i++ {
		if err := <-errors; err != nil {
			lastError = err
		}
	}
	if lastError != nil {
		return lastError
	}

	for tag, id := range tagsList {
		if askedTag != "" && tag != askedTag {
			continue
		}
		if err := s.Tag(repoInfo.LocalName, tag, id, true); err != nil {
			return err
		}
	}

	requestedTag := repoInfo.CanonicalName
	if len(askedTag) > 0 {
		requestedTag = utils.ImageReference(repoInfo.CanonicalName, askedTag)
	}
	WriteStatus(requestedTag, out, sf, layersDownloaded)
	return nil
}

func (s *TagStore) pullImage(r *registry.Session, out io.Writer, imgID, endpoint string, token []string, sf *streamformatter.StreamFormatter) (bool, error) {
	history, err := r.GetRemoteHistory(imgID, endpoint, token)
	if err != nil {
		return false, err
	}
	out.Write(sf.FormatProgress(stringid.TruncateID(imgID), "Pulling dependent layers", nil))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	layersDownloaded := false
	for i := len(history) - 1; i >= 0; i-- {
		id := history[i]

		// ensure no two downloads of the same layer happen at the same time
		if c, err := s.poolAdd("pull", "layer:"+id); err != nil {
			logrus.Debugf("Image (id: %s) pull is already running, skipping: %v", id, err)
			<-c
		}
		defer s.poolRemove("pull", "layer:"+id)

		if !s.graph.Exists(id) {
			out.Write(sf.FormatProgress(stringid.TruncateID(id), "Pulling metadata", nil))
			var (
				imgJSON []byte
				imgSize int
				err     error
				img     *image.Image
			)
			retries := 5
			for j := 1; j <= retries; j++ {
				imgJSON, imgSize, err = r.GetRemoteImageJSON(id, endpoint, token)
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, err
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				}
				img, err = image.NewImgJSON(imgJSON)
				layersDownloaded = true
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, fmt.Errorf("Failed to parse json: %s", err)
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else {
					break
				}
			}

			for j := 1; j <= retries; j++ {
				// Get the layer
				status := "Pulling fs layer"
				if j > 1 {
					status = fmt.Sprintf("Pulling fs layer [retries: %d]", j)
				}
				out.Write(sf.FormatProgress(stringid.TruncateID(id), status, nil))
				layer, err := r.GetRemoteImageLayer(img.ID, endpoint, token, int64(imgSize))
				if uerr, ok := err.(*url.Error); ok {
					err = uerr.Err
				}
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, err
				}
				layersDownloaded = true
				defer layer.Close()

				err = s.graph.Register(img,
					progressreader.New(progressreader.Config{
						In:        layer,
						Out:       out,
						Formatter: sf,
						Size:      imgSize,
						NewLines:  false,
						ID:        stringid.TruncateID(id),
						Action:    "Downloading",
					}))
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(stringid.TruncateID(id), "Error downloading dependent layers", nil))
					return layersDownloaded, err
				} else {
					break
				}
			}
		}
		out.Write(sf.FormatProgress(stringid.TruncateID(id), "Download complete", nil))
	}
	return layersDownloaded, nil
}

func WriteStatus(requestedTag string, out io.Writer, sf *streamformatter.StreamFormatter, layersDownloaded bool) {
	if layersDownloaded {
		out.Write(sf.FormatStatus("", "Status: Downloaded newer image for %s", requestedTag))
	} else {
		out.Write(sf.FormatStatus("", "Status: Image is up to date for %s", requestedTag))
	}
}

// downloadInfo is used to pass information from download to extractor
type downloadInfo struct {
	imgJSON    []byte
	img        *image.Image
	digest     digest.Digest
	tmpFile    *os.File
	length     int64
	downloaded bool
	err        chan error
}

func (s *TagStore) pullV2Repository(r *registry.Session, out io.Writer, repoInfo *registry.RepositoryInfo, tag string, sf *streamformatter.StreamFormatter) error {
	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			logrus.Debugf("Unable to pull from V2 registry, falling back to v1: %s", err)
			return ErrV2RegistryUnavailable
		}
		return fmt.Errorf("error getting registry endpoint: %s", err)
	}
	auth, err := r.GetV2Authorization(endpoint, repoInfo.RemoteName, true)
	if err != nil {
		return fmt.Errorf("error getting authorization: %s", err)
	}
	var layersDownloaded bool
	if tag == "" {
		logrus.Debugf("Pulling tag list from V2 registry for %s", repoInfo.CanonicalName)
		tags, err := r.GetV2RemoteTags(endpoint, repoInfo.RemoteName, auth)
		if err != nil {
			return err
		}
		if len(tags) == 0 {
			return registry.ErrDoesNotExist
		}
		for _, t := range tags {
			if downloaded, err := s.pullV2Tag(r, out, endpoint, repoInfo, t, sf, auth); err != nil {
				return err
			} else if downloaded {
				layersDownloaded = true
			}
		}
	} else {
		if downloaded, err := s.pullV2Tag(r, out, endpoint, repoInfo, tag, sf, auth); err != nil {
			return err
		} else if downloaded {
			layersDownloaded = true
		}
	}

	requestedTag := repoInfo.CanonicalName
	if len(tag) > 0 {
		requestedTag = utils.ImageReference(repoInfo.CanonicalName, tag)
	}
	WriteStatus(requestedTag, out, sf, layersDownloaded)
	return nil
}

func (s *TagStore) pullV2Tag(r *registry.Session, out io.Writer, endpoint *registry.Endpoint, repoInfo *registry.RepositoryInfo, tag string, sf *streamformatter.StreamFormatter, auth *registry.RequestAuthorization) (bool, error) {
	logrus.Debugf("Pulling tag from V2 registry: %q", tag)

	manifestBytes, manifestDigest, err := r.GetV2ImageManifest(endpoint, repoInfo.RemoteName, tag, auth)
	if err != nil {
		return false, err
	}

	// loadManifest ensures that the manifest payload has the expected digest
	// if the tag is a digest reference.
	manifest, verified, err := s.loadManifest(manifestBytes, manifestDigest, tag)
	if err != nil {
		return false, fmt.Errorf("error verifying manifest: %s", err)
	}

	if err := checkValidManifest(manifest); err != nil {
		return false, err
	}

	if verified {
		logrus.Printf("Image manifest for %s has been verified", utils.ImageReference(repoInfo.CanonicalName, tag))
	}
	out.Write(sf.FormatStatus(tag, "Pulling from %s", repoInfo.CanonicalName))

	downloads := make([]downloadInfo, len(manifest.FSLayers))

	for i := len(manifest.FSLayers) - 1; i >= 0; i-- {
		var (
			sumStr  = manifest.FSLayers[i].BlobSum
			imgJSON = []byte(manifest.History[i].V1Compatibility)
		)

		img, err := image.NewImgJSON(imgJSON)
		if err != nil {
			return false, fmt.Errorf("failed to parse json: %s", err)
		}
		downloads[i].img = img

		// Check if exists
		if s.graph.Exists(img.ID) {
			logrus.Debugf("Image already exists: %s", img.ID)
			continue
		}

		dgst, err := digest.ParseDigest(sumStr)
		if err != nil {
			return false, err
		}
		downloads[i].digest = dgst

		out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Pulling fs layer", nil))

		downloadFunc := func(di *downloadInfo) error {
			logrus.Debugf("pulling blob %q to V1 img %s", sumStr, img.ID)

			if c, err := s.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))
				} else {
					logrus.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
			} else {
				defer s.poolRemove("pull", "img:"+img.ID)
				tmpFile, err := ioutil.TempFile("", "GetV2ImageBlob")
				if err != nil {
					return err
				}

				r, l, err := r.GetV2ImageBlobReader(endpoint, repoInfo.RemoteName, di.digest, auth)
				if err != nil {
					return err
				}
				defer r.Close()

				verifier, err := digest.NewDigestVerifier(di.digest)
				if err != nil {
					return err
				}

				if _, err := io.Copy(tmpFile, progressreader.New(progressreader.Config{
					In:        ioutil.NopCloser(io.TeeReader(r, verifier)),
					Out:       out,
					Formatter: sf,
					Size:      int(l),
					NewLines:  false,
					ID:        stringid.TruncateID(img.ID),
					Action:    "Downloading",
				})); err != nil {
					return fmt.Errorf("unable to copy v2 image blob data: %s", err)
				}

				out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Verifying Checksum", nil))

				if !verifier.Verified() {
					logrus.Infof("Image verification failed: checksum mismatch for %q", di.digest.String())
					verified = false
				}

				out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))

				logrus.Debugf("Downloaded %s to tempfile %s", img.ID, tmpFile.Name())
				di.tmpFile = tmpFile
				di.length = l
				di.downloaded = true
			}
			di.imgJSON = imgJSON

			return nil
		}

		downloads[i].err = make(chan error)
		go func(di *downloadInfo) {
			di.err <- downloadFunc(di)
		}(&downloads[i])
	}

	var tagUpdated bool
	for i := len(downloads) - 1; i >= 0; i-- {
		d := &downloads[i]
		if d.err != nil {
			if err := <-d.err; err != nil {
				return false, err
			}
		}
		if d.downloaded {
			// if tmpFile is empty assume download and extracted elsewhere
			defer os.Remove(d.tmpFile.Name())
			defer d.tmpFile.Close()
			d.tmpFile.Seek(0, 0)
			if d.tmpFile != nil {
				err = s.graph.Register(d.img,
					progressreader.New(progressreader.Config{
						In:        d.tmpFile,
						Out:       out,
						Formatter: sf,
						Size:      int(d.length),
						ID:        stringid.TruncateID(d.img.ID),
						Action:    "Extracting",
					}))
				if err != nil {
					return false, err
				}

				// FIXME: Pool release here for parallel tag pull (ensures any downloads block until fully extracted)
			}
			out.Write(sf.FormatProgress(stringid.TruncateID(d.img.ID), "Pull complete", nil))
			tagUpdated = true
		} else {
			out.Write(sf.FormatProgress(stringid.TruncateID(d.img.ID), "Already exists", nil))
		}

	}

	// Check for new tag if no layers downloaded
	if !tagUpdated {
		repo, err := s.Get(repoInfo.LocalName)
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

	if verified && tagUpdated {
		out.Write(sf.FormatStatus(utils.ImageReference(repoInfo.CanonicalName, tag), "The image you are pulling has been verified. Important: image verification is a tech preview feature and should not be relied on to provide security."))
	}

	if manifestDigest != "" {
		out.Write(sf.FormatStatus("", "Digest: %s", manifestDigest))
	}

	if utils.DigestReference(tag) {
		if err = s.SetDigest(repoInfo.LocalName, tag, downloads[0].img.ID); err != nil {
			return false, err
		}
	} else {
		// only set the repository/tag -> image ID mapping when pulling by tag (i.e. not by digest)
		if err = s.Tag(repoInfo.LocalName, tag, downloads[0].img.ID, true); err != nil {
			return false, err
		}
	}

	return tagUpdated, nil
}
