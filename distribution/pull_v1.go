package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
)

type v1Puller struct {
	v1IDService *metadata.V1IDService
	endpoint    registry.APIEndpoint
	config      *ImagePullConfig
	sf          *streamformatter.StreamFormatter
	repoInfo    *registry.RepositoryInfo
	session     *registry.Session
}

func (p *v1Puller) Pull(ref reference.Named) (fallback bool, err error) {
	if _, isDigested := ref.(reference.Digested); isDigested {
		// Allowing fallback, because HTTPS v1 is before HTTP v2
		return true, registry.ErrNoSupport{errors.New("Cannot pull by digest with v1 registry")}
	}

	tlsConfig, err := p.config.RegistryService.TLSConfig(p.repoInfo.Index.Name)
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
	if err := p.pullRepository(ref); err != nil {
		// TODO(dmcgowan): Check if should fallback
		return false, err
	}
	out := p.config.OutStream
	out.Write(p.sf.FormatStatus("", "%s: this image was pulled from a legacy registry.  Important: This registry version will not be supported in future versions of docker.", p.repoInfo.CanonicalName.Name()))

	return false, nil
}

func (p *v1Puller) pullRepository(ref reference.Named) error {
	out := p.config.OutStream
	out.Write(p.sf.FormatStatus("", "Pulling repository %s", p.repoInfo.CanonicalName.Name()))

	repoData, err := p.session.GetRepositoryData(p.repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return fmt.Errorf("Error: image %s not found", p.repoInfo.RemoteName.Name())
		}
		// Unexpected HTTP error
		return err
	}

	logrus.Debugf("Retrieving the tag list")
	var tagsList map[string]string
	tagged, isTagged := ref.(reference.Tagged)
	if !isTagged {
		tagsList, err = p.session.GetRemoteTags(repoData.Endpoints, p.repoInfo.RemoteName)
	} else {
		var tagID string
		tagsList = make(map[string]string)
		tagID, err = p.session.GetRemoteTag(repoData.Endpoints, p.repoInfo.RemoteName, tagged.Tag())
		if err == registry.ErrRepoNotFound {
			return fmt.Errorf("Tag %s not found in repository %s", tagged.Tag(), p.repoInfo.CanonicalName.Name())
		}
		tagsList[tagged.Tag()] = tagID
	}
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

	errors := make(chan error)
	layerDownloaded := make(chan struct{})

	layersDownloaded := false
	var wg sync.WaitGroup
	for _, imgData := range repoData.ImgList {
		if isTagged && imgData.Tag != tagged.Tag() {
			continue
		}

		wg.Add(1)
		go func(img *registry.ImgData) {
			p.downloadImage(out, repoData, img, layerDownloaded, errors)
			wg.Done()
		}(imgData)
	}

	go func() {
		wg.Wait()
		close(errors)
	}()

	var lastError error
selectLoop:
	for {
		select {
		case err, ok := <-errors:
			if !ok {
				break selectLoop
			}
			lastError = err
		case <-layerDownloaded:
			layersDownloaded = true
		}
	}

	if lastError != nil {
		return lastError
	}

	localNameRef := p.repoInfo.LocalName
	if isTagged {
		localNameRef, err = reference.WithTag(localNameRef, tagged.Tag())
		if err != nil {
			localNameRef = p.repoInfo.LocalName
		}
	}
	writeStatus(localNameRef.String(), out, p.sf, layersDownloaded)
	return nil
}

func (p *v1Puller) downloadImage(out io.Writer, repoData *registry.RepositoryData, img *registry.ImgData, layerDownloaded chan struct{}, errors chan error) {
	if img.Tag == "" {
		logrus.Debugf("Image (id: %s) present in this repository but untagged, skipping", img.ID)
		return
	}

	localNameRef, err := reference.WithTag(p.repoInfo.LocalName, img.Tag)
	if err != nil {
		retErr := fmt.Errorf("Image (id: %s) has invalid tag: %s", img.ID, img.Tag)
		logrus.Debug(retErr.Error())
		errors <- retErr
	}

	if err := v1.ValidateID(img.ID); err != nil {
		errors <- err
		return
	}

	out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, p.repoInfo.CanonicalName.Name()), nil))
	success := false
	var lastErr error
	var isDownloaded bool
	for _, ep := range p.repoInfo.Index.Mirrors {
		ep += "v1/"
		out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, mirror: %s", img.Tag, p.repoInfo.CanonicalName.Name(), ep), nil))
		if isDownloaded, err = p.pullImage(out, img.ID, ep, localNameRef); err != nil {
			// Don't report errors when pulling from mirrors.
			logrus.Debugf("Error pulling image (%s) from %s, mirror: %s, %s", img.Tag, p.repoInfo.CanonicalName.Name(), ep, err)
			continue
		}
		if isDownloaded {
			layerDownloaded <- struct{}{}
		}
		success = true
		break
	}
	if !success {
		for _, ep := range repoData.Endpoints {
			out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, p.repoInfo.CanonicalName.Name(), ep), nil))
			if isDownloaded, err = p.pullImage(out, img.ID, ep, localNameRef); err != nil {
				// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
				// As the error is also given to the output stream the user will see the error.
				lastErr = err
				out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, endpoint: %s, %s", img.Tag, p.repoInfo.CanonicalName.Name(), ep, err), nil))
				continue
			}
			if isDownloaded {
				layerDownloaded <- struct{}{}
			}
			success = true
			break
		}
	}
	if !success {
		err := fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, p.repoInfo.CanonicalName.Name(), lastErr)
		out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), err.Error(), nil))
		errors <- err
		return
	}
	out.Write(p.sf.FormatProgress(stringid.TruncateID(img.ID), "Download complete", nil))
}

func (p *v1Puller) pullImage(out io.Writer, v1ID, endpoint string, localNameRef reference.Named) (layersDownloaded bool, err error) {
	var history []string
	history, err = p.session.GetRemoteHistory(v1ID, endpoint)
	if err != nil {
		return false, err
	}
	if len(history) < 1 {
		return false, fmt.Errorf("empty history for image %s", v1ID)
	}
	out.Write(p.sf.FormatProgress(stringid.TruncateID(v1ID), "Pulling dependent layers", nil))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	var (
		referencedLayers []layer.Layer
		parentID         layer.ChainID
		newHistory       []image.History
		img              *image.V1Image
		imgJSON          []byte
		imgSize          int64
	)

	defer func() {
		for _, l := range referencedLayers {
			layer.ReleaseAndLog(p.config.LayerStore, l)
		}
	}()

	layersDownloaded = false

	// Iterate over layers from top-most to bottom-most, checking if any
	// already exist on disk.
	var i int
	for i = 0; i != len(history); i++ {
		v1LayerID := history[i]
		// Do we have a mapping for this particular v1 ID on this
		// registry?
		if layerID, err := p.v1IDService.Get(v1LayerID, p.repoInfo.Index.Name); err == nil {
			// Does the layer actually exist
			if l, err := p.config.LayerStore.Get(layerID); err == nil {
				for j := i; j >= 0; j-- {
					logrus.Debugf("Layer already exists: %s", history[j])
					out.Write(p.sf.FormatProgress(stringid.TruncateID(history[j]), "Already exists", nil))
				}
				referencedLayers = append(referencedLayers, l)
				parentID = layerID
				break
			}
		}
	}

	needsDownload := i

	// Iterate over layers, in order from bottom-most to top-most. Download
	// config for all layers, and download actual layer data if needed.
	for i = len(history) - 1; i >= 0; i-- {
		v1LayerID := history[i]
		imgJSON, imgSize, err = p.downloadLayerConfig(out, v1LayerID, endpoint)
		if err != nil {
			return layersDownloaded, err
		}

		img = &image.V1Image{}
		if err := json.Unmarshal(imgJSON, img); err != nil {
			return layersDownloaded, err
		}

		if i < needsDownload {
			l, err := p.downloadLayer(out, v1LayerID, endpoint, parentID, imgSize, &layersDownloaded)

			// Note: This needs to be done even in the error case to avoid
			// stale references to the layer.
			if l != nil {
				referencedLayers = append(referencedLayers, l)
			}
			if err != nil {
				return layersDownloaded, err
			}

			parentID = l.ChainID()
		}

		// Create a new-style config from the legacy configs
		h, err := v1.HistoryFromConfig(imgJSON, false)
		if err != nil {
			return layersDownloaded, err
		}
		newHistory = append(newHistory, h)
	}

	rootFS := image.NewRootFS()
	l := referencedLayers[len(referencedLayers)-1]
	for l != nil {
		rootFS.DiffIDs = append([]layer.DiffID{l.DiffID()}, rootFS.DiffIDs...)
		l = l.Parent()
	}

	config, err := v1.MakeConfigFromV1Config(imgJSON, rootFS, newHistory)
	if err != nil {
		return layersDownloaded, err
	}

	imageID, err := p.config.ImageStore.Create(config)
	if err != nil {
		return layersDownloaded, err
	}

	if err := p.config.TagStore.AddTag(localNameRef, imageID, true); err != nil {
		return layersDownloaded, err
	}

	return layersDownloaded, nil
}

func (p *v1Puller) downloadLayerConfig(out io.Writer, v1LayerID, endpoint string) (imgJSON []byte, imgSize int64, err error) {
	out.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), "Pulling metadata", nil))

	retries := 5
	for j := 1; j <= retries; j++ {
		imgJSON, imgSize, err := p.session.GetRemoteImageJSON(v1LayerID, endpoint)
		if err != nil && j == retries {
			out.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), "Error pulling layer metadata", nil))
			return nil, 0, err
		} else if err != nil {
			time.Sleep(time.Duration(j) * 500 * time.Millisecond)
			continue
		}

		return imgJSON, imgSize, nil
	}

	// not reached
	return nil, 0, nil
}

func (p *v1Puller) downloadLayer(out io.Writer, v1LayerID, endpoint string, parentID layer.ChainID, layerSize int64, layersDownloaded *bool) (l layer.Layer, err error) {
	// ensure no two downloads of the same layer happen at the same time
	poolKey := "layer:" + v1LayerID
	broadcaster, found := p.config.Pool.add(poolKey)
	broadcaster.Add(out)
	if found {
		logrus.Debugf("Image (id: %s) pull is already running, skipping", v1LayerID)
		if err = broadcaster.Wait(); err != nil {
			return nil, err
		}
		layerID, err := p.v1IDService.Get(v1LayerID, p.repoInfo.Index.Name)
		if err != nil {
			return nil, err
		}
		// Does the layer actually exist
		l, err := p.config.LayerStore.Get(layerID)
		if err != nil {
			return nil, err
		}
		return l, nil
	}

	// This must use a closure so it captures the value of err when
	// the function returns, not when the 'defer' is evaluated.
	defer func() {
		p.config.Pool.removeWithError(poolKey, err)
	}()

	retries := 5
	for j := 1; j <= retries; j++ {
		// Get the layer
		status := "Pulling fs layer"
		if j > 1 {
			status = fmt.Sprintf("Pulling fs layer [retries: %d]", j)
		}
		broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), status, nil))
		layerReader, err := p.session.GetRemoteImageLayer(v1LayerID, endpoint, layerSize)
		if uerr, ok := err.(*url.Error); ok {
			err = uerr.Err
		}
		if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
			time.Sleep(time.Duration(j) * 500 * time.Millisecond)
			continue
		} else if err != nil {
			broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), "Error pulling dependent layers", nil))
			return nil, err
		}
		*layersDownloaded = true
		defer layerReader.Close()

		reader := progressreader.New(progressreader.Config{
			In:        layerReader,
			Out:       broadcaster,
			Formatter: p.sf,
			Size:      layerSize,
			NewLines:  false,
			ID:        stringid.TruncateID(v1LayerID),
			Action:    "Downloading",
		})

		inflatedLayerData, err := archive.DecompressStream(reader)
		if err != nil {
			return nil, fmt.Errorf("could not get decompression stream: %v", err)
		}

		l, err := p.config.LayerStore.Register(inflatedLayerData, parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to register layer: %v", err)
		}
		logrus.Debugf("layer %s registered successfully", l.DiffID())

		if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
			time.Sleep(time.Duration(j) * 500 * time.Millisecond)
			continue
		} else if err != nil {
			broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), "Error downloading dependent layers", nil))
			return nil, err
		}

		// Cache mapping from this v1 ID to content-addressable layer ID
		if err := p.v1IDService.Set(v1LayerID, p.repoInfo.Index.Name, l.ChainID()); err != nil {
			return nil, err
		}

		broadcaster.Write(p.sf.FormatProgress(stringid.TruncateID(v1LayerID), "Download complete", nil))
		broadcaster.Close()
		return l, nil
	}

	// not reached
	return nil, nil
}
