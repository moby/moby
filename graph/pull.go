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
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/transport"
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

	logName := repoInfo.LocalName
	if tag != "" {
		logName = utils.ImageReference(logName, tag)
	}

	// Attempt pulling official content from a provided v2 mirror
	if repoInfo.Index.Official {
		v2mirrorEndpoint, v2mirrorRepoInfo, err := configureV2Mirror(repoInfo, s.registryService)
		if err != nil {
			logrus.Errorf("Error configuring mirrors: %s", err)
			return err
		}

		if v2mirrorEndpoint != nil {
			logrus.Debugf("Attempting to pull from v2 mirror: %s", v2mirrorEndpoint.URL)
			return s.pullFromV2Mirror(v2mirrorEndpoint, v2mirrorRepoInfo, imagePullConfig, tag, sf, logName)
		}
	}

	logrus.Debugf("pulling image from host %q with remote name %q", repoInfo.Index.Name, repoInfo.RemoteName)

	endpoint, err := repoInfo.GetEndpoint(imagePullConfig.MetaHeaders)
	if err != nil {
		return err
	}
	// TODO(tiborvass): reuse client from endpoint?
	// Adds Docker-specific headers as well as user-specified headers (metaHeaders)
	tr := transport.NewTransport(
		registry.NewTransport(registry.ReceiveTimeout, endpoint.IsSecure),
		registry.DockerHeaders(imagePullConfig.MetaHeaders)...,
	)
	client := registry.HTTPClient(tr)
	r, err := registry.NewSession(client, imagePullConfig.AuthConfig, endpoint)
	if err != nil {
		return err
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

	if utils.DigestReference(tag) {
		return fmt.Errorf("pulling with digest reference failed from v2 registry")
	}

	logrus.Debugf("pulling v1 repository with local name %q", repoInfo.LocalName)
	if err = s.pullRepository(r, imagePullConfig.OutStream, repoInfo, tag, sf); err != nil {
		return err
	}

	s.eventsService.Log("pull", logName, "")

	return nil

}

func makeMirrorRepoInfo(repoInfo *registry.RepositoryInfo, mirror string) *registry.RepositoryInfo {
	mirrorRepo := &registry.RepositoryInfo{
		RemoteName:    repoInfo.RemoteName,
		LocalName:     repoInfo.LocalName,
		CanonicalName: repoInfo.CanonicalName,
		Official:      false,

		Index: &registry.IndexInfo{
			Official: false,
			Secure:   repoInfo.Index.Secure,
			Name:     mirror,
			Mirrors:  []string{},
		},
	}
	return mirrorRepo
}

func configureV2Mirror(repoInfo *registry.RepositoryInfo, s *registry.Service) (*registry.Endpoint, *registry.RepositoryInfo, error) {
	mirrors := repoInfo.Index.Mirrors
	if len(mirrors) == 0 {
		// no mirrors configured
		return nil, nil, nil
	}

	v1MirrorCount := 0
	var v2MirrorEndpoint *registry.Endpoint
	var v2MirrorRepoInfo *registry.RepositoryInfo
	for _, mirror := range mirrors {
		mirrorRepoInfo := makeMirrorRepoInfo(repoInfo, mirror)
		endpoint, err := registry.NewEndpoint(mirrorRepoInfo.Index, nil)
		if err != nil {
			logrus.Errorf("Unable to create endpoint for %s: %s", mirror, err)
			continue
		}
		if endpoint.Version == 2 {
			if v2MirrorEndpoint == nil {
				v2MirrorEndpoint = endpoint
				v2MirrorRepoInfo = mirrorRepoInfo
			} else {
				// > 1 v2 mirrors given
				return nil, nil, fmt.Errorf("multiple v2 mirrors configured")
			}
		} else {
			v1MirrorCount++
		}
	}

	if v1MirrorCount == len(mirrors) {
		// OK, but mirrors are v1
		return nil, nil, nil
	}
	if v2MirrorEndpoint != nil && v1MirrorCount == 0 {
		// OK, 1 v2 mirror specified
		return v2MirrorEndpoint, v2MirrorRepoInfo, nil
	}
	if v2MirrorEndpoint != nil && v1MirrorCount > 0 {
		return nil, nil, fmt.Errorf("v1 and v2 mirrors configured")
	}
	// No endpoint could be established with the given mirror configurations
	// Fallback to pulling from the hub as per v1 behavior.
	return nil, nil, nil
}

func (s *TagStore) pullFromV2Mirror(mirrorEndpoint *registry.Endpoint, repoInfo *registry.RepositoryInfo,
	imagePullConfig *ImagePullConfig, tag string, sf *streamformatter.StreamFormatter, logName string) error {

	tr := transport.NewTransport(
		registry.NewTransport(registry.ReceiveTimeout, mirrorEndpoint.IsSecure),
		registry.DockerHeaders(imagePullConfig.MetaHeaders)...,
	)
	client := registry.HTTPClient(tr)
	mirrorSession, err := registry.NewSession(client, &cliconfig.AuthConfig{}, mirrorEndpoint)
	if err != nil {
		return err
	}
	logrus.Debugf("Pulling v2 repository with local name %q from %s", repoInfo.LocalName, mirrorEndpoint.URL)
	if err := s.pullV2Repository(mirrorSession, imagePullConfig.OutStream, repoInfo, tag, sf); err != nil {
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
	tagsList := make(map[string]string)
	if askedTag == "" {
		tagsList, err = r.GetRemoteTags(repoData.Endpoints, repoInfo.RemoteName)
	} else {
		var tagId string
		tagId, err = r.GetRemoteTag(repoData.Endpoints, repoInfo.RemoteName, askedTag)
		tagsList[askedTag] = tagId
	}
	if err != nil {
		if err == registry.ErrRepoNotFound && askedTag != "" {
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, repoInfo.CanonicalName)
		}
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
				// Ensure endpoint is v1
				ep = ep + "v1/"
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
	history, err := r.GetRemoteHistory(imgID, endpoint)
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
				img     *Image
			)
			retries := 5
			for j := 1; j <= retries; j++ {
				imgJSON, imgSize, err = r.GetRemoteImageJSON(id, endpoint)
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, err
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				}
				img, err = NewImgJSON(imgJSON)
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
				layer, err := r.GetRemoteImageLayer(img.ID, endpoint, int64(imgSize))
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
	if !auth.CanAuthorizeV2() {
		return ErrV2RegistryUnavailable
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

	remoteDigest, manifestBytes, err := r.GetV2ImageManifest(endpoint, repoInfo.RemoteName, tag, auth)
	if err != nil {
		return false, err
	}

	// loadManifest ensures that the manifest payload has the expected digest
	// if the tag is a digest reference.
	localDigest, manifest, verified, err := s.loadManifest(manifestBytes, tag, remoteDigest)
	if err != nil {
		return false, fmt.Errorf("error verifying manifest: %s", err)
	}

	if verified {
		logrus.Printf("Image manifest for %s has been verified", utils.ImageReference(repoInfo.CanonicalName, tag))
	}
	out.Write(sf.FormatStatus(tag, "Pulling from %s", repoInfo.CanonicalName))

	// downloadInfo is used to pass information from download to extractor
	type downloadInfo struct {
		imgJSON    []byte
		img        *Image
		digest     digest.Digest
		tmpFile    *os.File
		length     int64
		downloaded bool
		err        chan error
	}

	downloads := make([]downloadInfo, len(manifest.FSLayers))

	for i := len(manifest.FSLayers) - 1; i >= 0; i-- {
		var (
			sumStr  = manifest.FSLayers[i].BlobSum
			imgJSON = []byte(manifest.History[i].V1Compatibility)
		)

		img, err := NewImgJSON(imgJSON)
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
					return fmt.Errorf("image layer digest verification failed for %q", di.digest)
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

				if err := s.graph.SetDigest(d.img.ID, d.digest); err != nil {
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

	if localDigest != remoteDigest { // this is not a verification check.
		// NOTE(stevvooe): This is a very defensive branch and should never
		// happen, since all manifest digest implementations use the same
		// algorithm.
		logrus.WithFields(
			logrus.Fields{
				"local":  localDigest,
				"remote": remoteDigest,
			}).Debugf("local digest does not match remote")

		out.Write(sf.FormatStatus("", "Remote Digest: %s", remoteDigest))
	}

	out.Write(sf.FormatStatus("", "Digest: %s", localDigest))

	if tag == localDigest.String() {
		// TODO(stevvooe): Ideally, we should always set the digest so we can
		// use the digest whether we pull by it or not. Unfortunately, the tag
		// store treats the digest as a separate tag, meaning there may be an
		// untagged digest image that would seem to be dangling by a user.

		if err = s.SetDigest(repoInfo.LocalName, localDigest.String(), downloads[0].img.ID); err != nil {
			return false, err
		}
	}

	if !utils.DigestReference(tag) {
		// only set the repository/tag -> image ID mapping when pulling by tag (i.e. not by digest)
		if err = s.Tag(repoInfo.LocalName, tag, downloads[0].img.ID, true); err != nil {
			return false, err
		}
	}

	return tagUpdated, nil
}
