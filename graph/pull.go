package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
)

func (s *TagStore) verifyManifest(eng *engine.Engine, manifestBytes []byte) (*registry.ManifestData, bool, error) {
	sig, err := libtrust.ParsePrettySignature(manifestBytes, "signatures")
	if err != nil {
		return nil, false, fmt.Errorf("error parsing payload: %s", err)
	}
	keys, err := sig.Verify()
	if err != nil {
		return nil, false, fmt.Errorf("error verifying payload: %s", err)
	}

	payload, err := sig.Payload()
	if err != nil {
		return nil, false, fmt.Errorf("error retrieving payload: %s", err)
	}

	var manifest registry.ManifestData
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, false, fmt.Errorf("error unmarshalling manifest: %s", err)
	}
	if manifest.SchemaVersion != 1 {
		return nil, false, fmt.Errorf("unsupported schema version: %d", manifest.SchemaVersion)
	}

	var verified bool
	for _, key := range keys {
		job := eng.Job("trust_key_check")
		b, err := key.MarshalJSON()
		if err != nil {
			return nil, false, fmt.Errorf("error marshalling public key: %s", err)
		}
		namespace := manifest.Name
		if namespace[0] != '/' {
			namespace = "/" + namespace
		}
		stdoutBuffer := bytes.NewBuffer(nil)

		job.Args = append(job.Args, namespace)
		job.Setenv("PublicKey", string(b))
		// Check key has read/write permission (0x03)
		job.SetenvInt("Permission", 0x03)
		job.Stdout.Add(stdoutBuffer)
		if err = job.Run(); err != nil {
			return nil, false, fmt.Errorf("error running key check: %s", err)
		}
		result := engine.Tail(stdoutBuffer, 1)
		log.Debugf("Key check result: %q", result)
		if result == "verified" {
			verified = true
		}
	}

	return &manifest, verified, nil
}

func (s *TagStore) CmdPull(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 && n != 2 {
		return job.Errorf("Usage: %s IMAGE [TAG]", job.Name)
	}

	var (
		localName   = job.Args[0]
		tag         string
		sf          = utils.NewStreamFormatter(job.GetenvBool("json"))
		authConfig  = &registry.AuthConfig{}
		metaHeaders map[string][]string
		mirrors     []string
	)

	if len(job.Args) > 1 {
		tag = job.Args[1]
	}

	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", &metaHeaders)

	c, err := s.poolAdd("pull", localName+":"+tag)
	if err != nil {
		if c != nil {
			// Another pull of the same repository is already taking place; just wait for it to finish
			job.Stdout.Write(sf.FormatStatus("", "Repository %s already being pulled by another client. Waiting.", localName))
			<-c
			return engine.StatusOK
		}
		return job.Error(err)
	}
	defer s.poolRemove("pull", localName+":"+tag)

	// Resolve the Repository name from fqn to endpoint + name
	hostname, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return job.Error(err)
	}

	endpoint, err := registry.NewEndpoint(hostname, s.insecureRegistries)
	if err != nil {
		return job.Error(err)
	}

	r, err := registry.NewSession(authConfig, registry.HTTPRequestFactory(metaHeaders), endpoint, true)
	if err != nil {
		return job.Error(err)
	}

	var isOfficial bool
	if endpoint.VersionString(1) == registry.IndexServerAddress() {
		// If pull "index.docker.io/foo/bar", it's stored locally under "foo/bar"
		localName = remoteName

		isOfficial = isOfficialName(remoteName)
		if isOfficial && strings.IndexRune(remoteName, '/') == -1 {
			remoteName = "library/" + remoteName
		}

		// Use provided mirrors, if any
		mirrors = s.mirrors
	}

	logName := localName
	if tag != "" {
		logName += ":" + tag
	}

	if len(mirrors) == 0 && (isOfficial || endpoint.Version == registry.APIVersion2) {
		j := job.Eng.Job("trust_update_base")
		if err = j.Run(); err != nil {
			return job.Errorf("error updating trust base graph: %s", err)
		}

		if err := s.pullV2Repository(job.Eng, r, job.Stdout, localName, remoteName, tag, sf, job.GetenvBool("parallel")); err == nil {
			if err = job.Eng.Job("log", "pull", logName, "").Run(); err != nil {
				log.Errorf("Error logging event 'pull' for %s: %s", logName, err)
			}
			return engine.StatusOK
		} else if err != registry.ErrDoesNotExist {
			log.Errorf("Error from V2 registry: %s", err)
		}
	}

	if err = s.pullRepository(r, job.Stdout, localName, remoteName, tag, sf, job.GetenvBool("parallel"), mirrors); err != nil {
		return job.Error(err)
	}

	if err = job.Eng.Job("log", "pull", logName, "").Run(); err != nil {
		log.Errorf("Error logging event 'pull' for %s: %s", logName, err)
	}

	return engine.StatusOK
}

func (s *TagStore) pullRepository(r *registry.Session, out io.Writer, localName, remoteName, askedTag string, sf *utils.StreamFormatter, parallel bool, mirrors []string) error {
	out.Write(sf.FormatStatus("", "Pulling repository %s", localName))

	repoData, err := r.GetRepositoryData(remoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return fmt.Errorf("Error: image %s:%s not found", remoteName, askedTag)
		}
		// Unexpected HTTP error
		return err
	}

	log.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, remoteName, repoData.Tokens)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	for tag, id := range tagsList {
		repoData.ImgList[id] = &registry.ImgData{
			ID:       id,
			Tag:      tag,
			Checksum: "",
		}
	}

	log.Debugf("Registering tags")
	// If no tag has been specified, pull them all
	var imageId string
	if askedTag == "" {
		for tag, id := range tagsList {
			repoData.ImgList[id].Tag = tag
		}
	} else {
		// Otherwise, check that the tag exists and use only that one
		id, exists := tagsList[askedTag]
		if !exists {
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, localName)
		}
		imageId = id
		repoData.ImgList[id].Tag = askedTag
	}

	errors := make(chan error)

	layers_downloaded := false
	for _, image := range repoData.ImgList {
		downloadImage := func(img *registry.ImgData) {
			if askedTag != "" && img.Tag != askedTag {
				log.Debugf("(%s) does not match %s (id: %s), skipping", img.Tag, askedTag, img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			if img.Tag == "" {
				log.Debugf("Image (id: %s) present in this repository but untagged, skipping", img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			// ensure no two downloads of the same image happen at the same time
			if c, err := s.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))
				} else {
					log.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
				if parallel {
					errors <- nil
				}
				return
			}
			defer s.poolRemove("pull", "img:"+img.ID)

			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, localName), nil))
			success := false
			var lastErr, err error
			var is_downloaded bool
			if mirrors != nil {
				for _, ep := range mirrors {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, mirror: %s", img.Tag, localName, ep), nil))
					if is_downloaded, err = s.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
						// Don't report errors when pulling from mirrors.
						log.Debugf("Error pulling image (%s) from %s, mirror: %s, %s", img.Tag, localName, ep, err)
						continue
					}
					layers_downloaded = layers_downloaded || is_downloaded
					success = true
					break
				}
			}
			if !success {
				for _, ep := range repoData.Endpoints {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, localName, ep), nil))
					if is_downloaded, err = s.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
						// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
						// As the error is also given to the output stream the user will see the error.
						lastErr = err
						out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, endpoint: %s, %s", img.Tag, localName, ep, err), nil))
						continue
					}
					layers_downloaded = layers_downloaded || is_downloaded
					success = true
					break
				}
			}
			if !success {
				err := fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, localName, lastErr)
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), err.Error(), nil))
				if parallel {
					errors <- err
					return
				}
			}
			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))

			if parallel {
				errors <- nil
			}
		}

		if parallel {
			go downloadImage(image)
		} else {
			downloadImage(image)
		}
	}
	if parallel {
		var lastError error
		for i := 0; i < len(repoData.ImgList); i++ {
			if err := <-errors; err != nil {
				lastError = err
			}
		}
		if lastError != nil {
			return lastError
		}

	}
	for tag, id := range tagsList {
		if askedTag != "" && id != imageId {
			continue
		}
		if err := s.Set(localName, tag, id, true); err != nil {
			return err
		}
	}

	requestedTag := localName
	if len(askedTag) > 0 {
		requestedTag = localName + ":" + askedTag
	}
	WriteStatus(requestedTag, out, sf, layers_downloaded)
	return nil
}

func (s *TagStore) pullImage(r *registry.Session, out io.Writer, imgID, endpoint string, token []string, sf *utils.StreamFormatter) (bool, error) {
	history, err := r.GetRemoteHistory(imgID, endpoint, token)
	if err != nil {
		return false, err
	}
	out.Write(sf.FormatProgress(utils.TruncateID(imgID), "Pulling dependent layers", nil))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	layers_downloaded := false
	for i := len(history) - 1; i >= 0; i-- {
		id := history[i]

		// ensure no two downloads of the same layer happen at the same time
		if c, err := s.poolAdd("pull", "layer:"+id); err != nil {
			log.Debugf("Image (id: %s) pull is already running, skipping: %v", id, err)
			<-c
		}
		defer s.poolRemove("pull", "layer:"+id)

		if !s.graph.Exists(id) {
			out.Write(sf.FormatProgress(utils.TruncateID(id), "Pulling metadata", nil))
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
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return layers_downloaded, err
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				}
				img, err = image.NewImgJSON(imgJSON)
				layers_downloaded = true
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return layers_downloaded, fmt.Errorf("Failed to parse json: %s", err)
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
				out.Write(sf.FormatProgress(utils.TruncateID(id), status, nil))
				layer, err := r.GetRemoteImageLayer(img.ID, endpoint, token, int64(imgSize))
				if uerr, ok := err.(*url.Error); ok {
					err = uerr.Err
				}
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return layers_downloaded, err
				}
				layers_downloaded = true
				defer layer.Close()

				err = s.graph.Register(img,
					utils.ProgressReader(layer, imgSize, out, sf, false, utils.TruncateID(id), "Downloading"))
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error downloading dependent layers", nil))
					return layers_downloaded, err
				} else {
					break
				}
			}
		}
		out.Write(sf.FormatProgress(utils.TruncateID(id), "Download complete", nil))
	}
	return layers_downloaded, nil
}

func WriteStatus(requestedTag string, out io.Writer, sf *utils.StreamFormatter, layers_downloaded bool) {
	if layers_downloaded {
		out.Write(sf.FormatStatus("", "Status: Downloaded newer image for %s", requestedTag))
	} else {
		out.Write(sf.FormatStatus("", "Status: Image is up to date for %s", requestedTag))
	}
}

// downloadInfo is used to pass information from download to extractor
type downloadInfo struct {
	imgJSON    []byte
	img        *image.Image
	tmpFile    *os.File
	length     int64
	downloaded bool
	err        chan error
}

func (s *TagStore) pullV2Repository(eng *engine.Engine, r *registry.Session, out io.Writer, localName, remoteName, tag string, sf *utils.StreamFormatter, parallel bool) error {
	var layersDownloaded bool
	if tag == "" {
		log.Debugf("Pulling tag list from V2 registry for %s", remoteName)
		tags, err := r.GetV2RemoteTags(remoteName, nil)
		if err != nil {
			return err
		}
		for _, t := range tags {
			if downloaded, err := s.pullV2Tag(eng, r, out, localName, remoteName, t, sf, parallel); err != nil {
				return err
			} else if downloaded {
				layersDownloaded = true
			}
		}
	} else {
		if downloaded, err := s.pullV2Tag(eng, r, out, localName, remoteName, tag, sf, parallel); err != nil {
			return err
		} else if downloaded {
			layersDownloaded = true
		}
	}

	requestedTag := localName
	if len(tag) > 0 {
		requestedTag = localName + ":" + tag
	}
	WriteStatus(requestedTag, out, sf, layersDownloaded)
	return nil
}

func (s *TagStore) pullV2Tag(eng *engine.Engine, r *registry.Session, out io.Writer, localName, remoteName, tag string, sf *utils.StreamFormatter, parallel bool) (bool, error) {
	log.Debugf("Pulling tag from V2 registry: %q", tag)
	manifestBytes, err := r.GetV2ImageManifest(remoteName, tag, nil)
	if err != nil {
		return false, err
	}

	manifest, verified, err := s.verifyManifest(eng, manifestBytes)
	if err != nil {
		return false, fmt.Errorf("error verifying manifest: %s", err)
	}

	if len(manifest.FSLayers) != len(manifest.History) {
		return false, fmt.Errorf("length of history not equal to number of layers")
	}

	if verified {
		out.Write(sf.FormatStatus(localName+":"+tag, "The image you are pulling has been verified"))
	} else {
		out.Write(sf.FormatStatus(tag, "Pulling from %s", localName))
	}

	if len(manifest.FSLayers) == 0 {
		return false, fmt.Errorf("no blobSums in manifest")
	}

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
			log.Debugf("Image already exists: %s", img.ID)
			continue
		}

		chunks := strings.SplitN(sumStr, ":", 2)
		if len(chunks) < 2 {
			return false, fmt.Errorf("expected 2 parts in the sumStr, got %#v", chunks)
		}
		sumType, checksum := chunks[0], chunks[1]
		out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Pulling fs layer", nil))

		downloadFunc := func(di *downloadInfo) error {
			log.Debugf("pulling blob %q to V1 img %s", sumStr, img.ID)

			if c, err := s.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))
				} else {
					log.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
			} else {
				defer s.poolRemove("pull", "img:"+img.ID)
				tmpFile, err := ioutil.TempFile("", "GetV2ImageBlob")
				if err != nil {
					return err
				}

				r, l, err := r.GetV2ImageBlobReader(remoteName, sumType, checksum, nil)
				if err != nil {
					return err
				}
				defer r.Close()
				io.Copy(tmpFile, utils.ProgressReader(r, int(l), out, sf, false, utils.TruncateID(img.ID), "Downloading"))

				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))

				log.Debugf("Downloaded %s to tempfile %s", img.ID, tmpFile.Name())
				di.tmpFile = tmpFile
				di.length = l
				di.downloaded = true
			}
			di.imgJSON = imgJSON

			return nil
		}

		if parallel {
			downloads[i].err = make(chan error)
			go func(di *downloadInfo) {
				di.err <- downloadFunc(di)
			}(&downloads[i])
		} else {
			err := downloadFunc(&downloads[i])
			if err != nil {
				return false, err
			}
		}
	}

	var layersDownloaded bool
	for i := len(downloads) - 1; i >= 0; i-- {
		d := &downloads[i]
		if d.err != nil {
			err := <-d.err
			if err != nil {
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
					utils.ProgressReader(d.tmpFile, int(d.length), out, sf, false, utils.TruncateID(d.img.ID), "Extracting"))
				if err != nil {
					return false, err
				}

				// FIXME: Pool release here for parallel tag pull (ensures any downloads block until fully extracted)
			}
			out.Write(sf.FormatProgress(utils.TruncateID(d.img.ID), "Pull complete", nil))
			layersDownloaded = true
		} else {
			out.Write(sf.FormatProgress(utils.TruncateID(d.img.ID), "Already exists", nil))
		}

	}

	if err = s.Set(localName, tag, downloads[0].img.ID, true); err != nil {
		return false, err
	}

	return layersDownloaded, nil
}
