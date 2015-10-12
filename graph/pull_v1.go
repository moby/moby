package graph

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

type v1Puller struct {
	*TagStore
	endpoint registry.APIEndpoint
	config   *ImagePullConfig
	sf       *streamformatter.StreamFormatter
	repoInfo *registry.RepositoryInfo
	session  *registry.Session
}

func (p *v1Puller) Pull(tag string) (fallback bool, err error) {
	if utils.DigestReference(tag) {
		// Allowing fallback, because HTTPS v1 is before HTTP v2
		return true, registry.ErrNoSupport{errors.New("Cannot pull by digest with v1 registry")}
	}

	tlsConfig, err := p.registryService.TLSConfig(p.repoInfo.Index.Name)
	if err != nil {
		return false, err
	}
	// Adds Docker-specific headers as well as user-specified headers (metaHeaders)
	tr := transport.NewTransport(
		// TODO(tiborvass): was ReceiveTimeout
		registry.NewTransport(tlsConfig),
		registry.DockerHeaders(p.config.MetaHeaders)...,
	)
	client := registry.HTTPClient(tr)
	v1Endpoint, err := p.endpoint.ToV1Endpoint(p.config.MetaHeaders)
	if err != nil {
		logrus.Debugf("Could not get v1 endpoint: %v", err)
		return true, err
	}
	p.session, err = registry.NewSession(client, p.config.AuthConfig, v1Endpoint)
	if err != nil {
		// TODO(dmcgowan): Check if should fallback
		logrus.Debugf("Fallback from error: %s", err)
		return true, err
	}
	if err := p.pullRepository(tag); err != nil {
		// TODO(dmcgowan): Check if should fallback
		return false, err
	}
	out := p.config.OutStream
	out.Write(p.sf.FormatStatus("", "%s: this image was pulled from a legacy registry.  Important: This registry version will not be supported in future versions of docker.", p.repoInfo.CanonicalName))

	return false, nil
}

func (p *v1Puller) pullRepository(askedTag string) error {
	out := p.config.OutStream
	out.Write(p.sf.FormatStatus("", "Pulling repository %s", p.repoInfo.CanonicalName))

	repoData, err := p.session.GetRepositoryData(p.repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return fmt.Errorf("Error: image %s not found", utils.ImageReference(p.repoInfo.RemoteName, askedTag))
		}
		// Unexpected HTTP error
		return err
	}

	logrus.Debugf("Retrieving the tag list")
	tagsList := make(map[string]string)
	if askedTag == "" {
		tagsList, err = p.session.GetRemoteTags(repoData.Endpoints, p.repoInfo.RemoteName)
	} else {
		var tagID string
		tagID, err = p.session.GetRemoteTag(repoData.Endpoints, p.repoInfo.RemoteName, askedTag)
		tagsList[askedTag] = tagID
	}
	if err != nil {
		if err == registry.ErrRepoNotFound && askedTag != "" {
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, p.repoInfo.CanonicalName)
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
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, p.repoInfo.CanonicalName)
		}
		repoData.ImgList[id].Tag = askedTag
	}

	errors := make(chan error)

	layersDownloaded := false
	imgIDs := []string{}
	sessionID := p.session.ID()
	defer func() {
		p.graph.Release(sessionID, imgIDs...)
	}()
	for _, imgData := range repoData.ImgList {
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

			if err := image.ValidateID(img.ID); err != nil {
				errors <- err
				return
			}

			// ensure no two downloads of the same image happen at the same time
			poolKey := "img:" + img.ID
			broadcaster, found := p.poolAdd("pull", poolKey)
			broadcaster.Add(out)
			if found {
				errors <- broadcaster.Wait()
				return
			}
			defer p.poolRemove("pull", poolKey)

			// we need to retain it until tagging
			p.graph.Retain(sessionID, img.ID)
			imgIDs = append(imgIDs, img.ID)

			broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, p.repoInfo.CanonicalName), nil))
			success := false
			var lastErr, err error
			var isDownloaded bool
			for _, ep := range p.repoInfo.Index.Mirrors {
				ep += "v1/"
				broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, mirror: %s", img.Tag, p.repoInfo.CanonicalName, ep), nil))
				if isDownloaded, err = p.pullImage(broadcaster, img.ID, ep); err != nil {
					// Don't report errors when pulling from mirrors.
					logrus.Debugf("Error pulling image (%s) from %s, mirror: %s, %s", img.Tag, p.repoInfo.CanonicalName, ep, err)
					continue
				}
				layersDownloaded = layersDownloaded || isDownloaded
				success = true
				break
			}
			if !success {
				for _, ep := range repoData.Endpoints {
					broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, p.repoInfo.CanonicalName, ep), nil))
					if isDownloaded, err = p.pullImage(broadcaster, img.ID, ep); err != nil {
						// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
						// As the error is also given to the output stream the user will see the error.
						lastErr = err
						broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, endpoint: %s, %s", img.Tag, p.repoInfo.CanonicalName, ep, err), nil))
						continue
					}
					layersDownloaded = layersDownloaded || isDownloaded
					success = true
					break
				}
			}
			if !success {
				err := fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, p.repoInfo.CanonicalName, lastErr)
				broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), err.Error(), nil))
				errors <- err
				broadcaster.CloseWithError(err)
				return
			}
			broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))

			errors <- nil
		}

		go downloadImage(imgData)
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
		if err := p.Tag(p.repoInfo.LocalName, tag, id, true); err != nil {
			return err
		}
	}

	requestedTag := p.repoInfo.LocalName
	if len(askedTag) > 0 {
		requestedTag = utils.ImageReference(p.repoInfo.LocalName, askedTag)
	}
	writeStatus(requestedTag, out, p.sf, layersDownloaded)
	return nil
}

func (p *v1Puller) pullImage(out io.Writer, imgID, endpoint string) (layersDownloaded bool, err error) {
	var history []string
	history, err = p.session.GetRemoteHistory(imgID, endpoint)
	if err != nil {
		return false, err
	}
	out.Write(p.sf.FormatProgress(stringid.TruncateID(imgID), "Pulling dependent layers", nil))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	sessionID := p.session.ID()
	// As imgID has been retained in pullRepository, no need to retain again
	p.graph.Retain(sessionID, history[1:]...)
	defer p.graph.Release(sessionID, history[1:]...)

	layersDownloaded = false
	for i := len(history) - 1; i >= 0; i-- {
		id := history[i]

		// ensure no two downloads of the same layer happen at the same time
		poolKey := "layer:" + id
		broadcaster, found := p.poolAdd("pull", poolKey)
		broadcaster.Add(out)
		if found {
			logrus.Debugf("Image (id: %s) pull is already running, skipping", id)
			err = broadcaster.Wait()
			if err != nil {
				return layersDownloaded, err
			}
			continue
		}

		// This must use a closure so it captures the value of err when
		// the function returns, not when the 'defer' is evaluated.
		defer func() {
			p.poolRemoveWithError("pull", poolKey, err)
		}()

		if !p.graph.Exists(id) {
			broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Pulling metadata", nil))
			var (
				imgJSON []byte
				imgSize int64
				err     error
				img     *image.Image
			)
			retries := 5
			for j := 1; j <= retries; j++ {
				imgJSON, imgSize, err = p.session.GetRemoteImageJSON(id, endpoint)
				if err != nil && j == retries {
					broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, err
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				}
				img, err = image.NewImgJSON(imgJSON)
				layersDownloaded = true
				if err != nil && j == retries {
					broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
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
				broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), status, nil))
				layer, err := p.session.GetRemoteImageLayer(img.ID, endpoint, imgSize)
				if uerr, ok := err.(*url.Error); ok {
					err = uerr.Err
				}
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Error pulling dependent layers", nil))
					return layersDownloaded, err
				}
				layersDownloaded = true
				defer layer.Close()

				err = p.graph.Register(v1Descriptor{img},
					progressreader.New(progressreader.Config{
						In:        layer,
						Out:       broadcaster,
						Formatter: p.sf,
						Size:      imgSize,
						NewLines:  false,
						ID:        stringid.TruncateID(id),
						Action:    "Downloading",
					}))
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Error downloading dependent layers", nil))
					return layersDownloaded, err
				} else {
					break
				}
			}
		}
		broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(id), "Download complete", nil))
		broadcaster.Close()
	}
	return layersDownloaded, nil
}
