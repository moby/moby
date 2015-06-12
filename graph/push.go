package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/transport"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
)

var ErrV2RegistryUnavailable = errors.New("error v2 registry unavailable")

type ImagePushConfig struct {
	MetaHeaders map[string][]string
	AuthConfig  *cliconfig.AuthConfig
	Tag         string
	OutStream   io.Writer
}

// Retrieve the all the images to be uploaded in the correct order
func (s *TagStore) getImageList(localRepo map[string]string, requestedTag string) ([]string, map[string][]string, error) {
	var (
		imageList   []string
		imagesSeen  = make(map[string]bool)
		tagsByImage = make(map[string][]string)
	)

	for tag, id := range localRepo {
		if requestedTag != "" && requestedTag != tag {
			// Include only the requested tag.
			continue
		}

		if utils.DigestReference(tag) {
			// Ignore digest references.
			continue
		}

		var imageListForThisTag []string

		tagsByImage[id] = append(tagsByImage[id], tag)

		for img, err := s.graph.Get(id); img != nil; img, err = s.graph.GetParent(img) {
			if err != nil {
				return nil, nil, err
			}

			if imagesSeen[img.ID] {
				// This image is already on the list, we can ignore it and all its parents
				break
			}

			imagesSeen[img.ID] = true
			imageListForThisTag = append(imageListForThisTag, img.ID)
		}

		// reverse the image list for this tag (so the "most"-parent image is first)
		for i, j := 0, len(imageListForThisTag)-1; i < j; i, j = i+1, j-1 {
			imageListForThisTag[i], imageListForThisTag[j] = imageListForThisTag[j], imageListForThisTag[i]
		}

		// append to main image list
		imageList = append(imageList, imageListForThisTag...)
	}
	if len(imageList) == 0 {
		return nil, nil, fmt.Errorf("No images found for the requested repository / tag")
	}
	logrus.Debugf("Image list: %v", imageList)
	logrus.Debugf("Tags by image: %v", tagsByImage)

	return imageList, tagsByImage, nil
}

func (s *TagStore) getImageTags(localRepo map[string]string, askedTag string) ([]string, error) {
	logrus.Debugf("Checking %s against %#v", askedTag, localRepo)
	if len(askedTag) > 0 {
		if _, ok := localRepo[askedTag]; !ok || utils.DigestReference(askedTag) {
			return nil, fmt.Errorf("Tag does not exist: %s", askedTag)
		}
		return []string{askedTag}, nil
	}
	var tags []string
	for tag := range localRepo {
		if !utils.DigestReference(tag) {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// createImageIndex returns an index of an image's layer IDs and tags.
func (s *TagStore) createImageIndex(images []string, tags map[string][]string) []*registry.ImgData {
	var imageIndex []*registry.ImgData
	for _, id := range images {
		if tags, hasTags := tags[id]; hasTags {
			// If an image has tags you must add an entry in the image index
			// for each tag
			for _, tag := range tags {
				imageIndex = append(imageIndex, &registry.ImgData{
					ID:  id,
					Tag: tag,
				})
			}
			continue
		}
		// If the image does not have a tag it still needs to be sent to the
		// registry with an empty tag so that it is accociated with the repository
		imageIndex = append(imageIndex, &registry.ImgData{
			ID:  id,
			Tag: "",
		})
	}
	return imageIndex
}

type imagePushData struct {
	id       string
	endpoint string
	tokens   []string
}

// lookupImageOnEndpoint checks the specified endpoint to see if an image exists
// and if it is absent then it sends the image id to the channel to be pushed.
func lookupImageOnEndpoint(wg *sync.WaitGroup, r *registry.Session, out io.Writer, sf *streamformatter.StreamFormatter,
	images chan imagePushData, imagesToPush chan string) {
	defer wg.Done()
	for image := range images {
		if err := r.LookupRemoteImage(image.id, image.endpoint); err != nil {
			logrus.Errorf("Error in LookupRemoteImage: %s", err)
			imagesToPush <- image.id
			continue
		}
		out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", stringid.TruncateID(image.id)))
	}
}

func (s *TagStore) pushImageToEndpoint(endpoint string, out io.Writer, remoteName string, imageIDs []string,
	tags map[string][]string, repo *registry.RepositoryData, sf *streamformatter.StreamFormatter, r *registry.Session) error {
	workerCount := len(imageIDs)
	// start a maximum of 5 workers to check if images exist on the specified endpoint.
	if workerCount > 5 {
		workerCount = 5
	}
	var (
		wg           = &sync.WaitGroup{}
		imageData    = make(chan imagePushData, workerCount*2)
		imagesToPush = make(chan string, workerCount*2)
		pushes       = make(chan map[string]struct{}, 1)
	)
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go lookupImageOnEndpoint(wg, r, out, sf, imageData, imagesToPush)
	}
	// start a go routine that consumes the images to push
	go func() {
		shouldPush := make(map[string]struct{})
		for id := range imagesToPush {
			shouldPush[id] = struct{}{}
		}
		pushes <- shouldPush
	}()
	for _, id := range imageIDs {
		imageData <- imagePushData{
			id:       id,
			endpoint: endpoint,
			tokens:   repo.Tokens,
		}
	}
	// close the channel to notify the workers that there will be no more images to check.
	close(imageData)
	wg.Wait()
	close(imagesToPush)
	// wait for all the images that require pushes to be collected into a consumable map.
	shouldPush := <-pushes
	// finish by pushing any images and tags to the endpoint.  The order that the images are pushed
	// is very important that is why we are still iterating over the ordered list of imageIDs.
	for _, id := range imageIDs {
		if _, push := shouldPush[id]; push {
			if _, err := s.pushImage(r, out, id, endpoint, repo.Tokens, sf); err != nil {
				// FIXME: Continue on error?
				return err
			}
		}
		for _, tag := range tags[id] {
			out.Write(sf.FormatStatus("", "Pushing tag for rev [%s] on {%s}", stringid.TruncateID(id), endpoint+"repositories/"+remoteName+"/tags/"+tag))
			if err := r.PushRegistryTag(remoteName, id, tag, endpoint); err != nil {
				return err
			}
		}
	}
	return nil
}

// pushRepository pushes layers that do not already exist on the registry.
func (s *TagStore) pushRepository(r *registry.Session, out io.Writer,
	repoInfo *registry.RepositoryInfo, localRepo map[string]string,
	tag string, sf *streamformatter.StreamFormatter) error {
	logrus.Debugf("Local repo: %s", localRepo)
	out = ioutils.NewWriteFlusher(out)
	imgList, tags, err := s.getImageList(localRepo, tag)
	if err != nil {
		return err
	}
	out.Write(sf.FormatStatus("", "Sending image list"))

	imageIndex := s.createImageIndex(imgList, tags)
	logrus.Debugf("Preparing to push %s with the following images and tags", localRepo)
	for _, data := range imageIndex {
		logrus.Debugf("Pushing ID: %s with Tag: %s", data.ID, data.Tag)
	}
	// Register all the images in a repository with the registry
	// If an image is not in this list it will not be associated with the repository
	repoData, err := r.PushImageJSONIndex(repoInfo.RemoteName, imageIndex, false, nil)
	if err != nil {
		return err
	}
	nTag := 1
	if tag == "" {
		nTag = len(localRepo)
	}
	out.Write(sf.FormatStatus("", "Pushing repository %s (%d tags)", repoInfo.CanonicalName, nTag))
	// push the repository to each of the endpoints only if it does not exist.
	for _, endpoint := range repoData.Endpoints {
		if err := s.pushImageToEndpoint(endpoint, out, repoInfo.RemoteName, imgList, tags, repoData, sf, r); err != nil {
			return err
		}
	}
	_, err = r.PushImageJSONIndex(repoInfo.RemoteName, imageIndex, true, repoData.Endpoints)
	return err
}

func (s *TagStore) pushImage(r *registry.Session, out io.Writer, imgID, ep string, token []string, sf *streamformatter.StreamFormatter) (checksum string, err error) {
	out = ioutils.NewWriteFlusher(out)
	jsonRaw, err := s.graph.RawJSON(imgID)
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve the path for {%s}: %s", imgID, err)
	}
	out.Write(sf.FormatProgress(stringid.TruncateID(imgID), "Pushing", nil))

	imgData := &registry.ImgData{
		ID: imgID,
	}

	// Send the json
	if err := r.PushImageJSONRegistry(imgData, jsonRaw, ep); err != nil {
		if err == registry.ErrAlreadyExists {
			out.Write(sf.FormatProgress(stringid.TruncateID(imgData.ID), "Image already pushed, skipping", nil))
			return "", nil
		}
		return "", err
	}

	layerData, err := s.graph.TempLayerArchive(imgID, sf, out)
	if err != nil {
		return "", fmt.Errorf("Failed to generate layer archive: %s", err)
	}
	defer os.RemoveAll(layerData.Name())

	// Send the layer
	logrus.Debugf("rendered layer for %s of [%d] size", imgData.ID, layerData.Size)

	checksum, checksumPayload, err := r.PushImageLayerRegistry(imgData.ID,
		progressreader.New(progressreader.Config{
			In:        layerData,
			Out:       out,
			Formatter: sf,
			Size:      int(layerData.Size),
			NewLines:  false,
			ID:        stringid.TruncateID(imgData.ID),
			Action:    "Pushing",
		}), ep, jsonRaw)
	if err != nil {
		return "", err
	}
	imgData.Checksum = checksum
	imgData.ChecksumPayload = checksumPayload
	// Send the checksum
	if err := r.PushImageChecksumRegistry(imgData, ep); err != nil {
		return "", err
	}

	out.Write(sf.FormatProgress(stringid.TruncateID(imgData.ID), "Image successfully pushed", nil))
	return imgData.Checksum, nil
}

func (s *TagStore) pushV2Repository(r *registry.Session, localRepo Repository, out io.Writer, repoInfo *registry.RepositoryInfo, tag string, sf *streamformatter.StreamFormatter) error {
	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			logrus.Debugf("Unable to push to V2 registry, falling back to v1: %s", err)
			return ErrV2RegistryUnavailable
		}
		return fmt.Errorf("error getting registry endpoint: %s", err)
	}

	tags, err := s.getImageTags(localRepo, tag)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return fmt.Errorf("No tags to push for %s", repoInfo.LocalName)
	}

	auth, err := r.GetV2Authorization(endpoint, repoInfo.RemoteName, false)
	if err != nil {
		return fmt.Errorf("error getting authorization: %s", err)
	}

	for _, tag := range tags {
		logrus.Debugf("Pushing repository: %s:%s", repoInfo.CanonicalName, tag)

		layerId, exists := localRepo[tag]
		if !exists {
			return fmt.Errorf("tag does not exist: %s", tag)
		}

		layer, err := s.graph.Get(layerId)
		if err != nil {
			return err
		}

		m := &registry.ManifestData{
			SchemaVersion: 1,
			Name:          repoInfo.RemoteName,
			Tag:           tag,
			Architecture:  layer.Architecture,
		}
		var metadata runconfig.Config
		if layer.Config != nil {
			metadata = *layer.Config
		}

		layersSeen := make(map[string]bool)
		layers := []*image.Image{layer}
		for ; layer != nil; layer, err = s.graph.GetParent(layer) {
			if err != nil {
				return err
			}

			if layersSeen[layer.ID] {
				break
			}
			layers = append(layers, layer)
			layersSeen[layer.ID] = true
		}
		m.FSLayers = make([]*registry.FSLayer, len(layers))
		m.History = make([]*registry.ManifestHistory, len(layers))

		// Schema version 1 requires layer ordering from top to root
		for i, layer := range layers {
			logrus.Debugf("Pushing layer: %s", layer.ID)

			if layer.Config != nil && metadata.Image != layer.ID {
				if err := runconfig.Merge(&metadata, layer.Config); err != nil {
					return err
				}
			}
			jsonData, err := s.graph.RawJSON(layer.ID)
			if err != nil {
				return fmt.Errorf("cannot retrieve the path for %s: %s", layer.ID, err)
			}

			var exists bool
			dgst, err := s.graph.GetDigest(layer.ID)
			if err != nil {
				if err != ErrDigestNotSet {
					return fmt.Errorf("error getting image checksum: %s", err)
				}
			} else {
				// Call mount blob
				exists, err = r.HeadV2ImageBlob(endpoint, repoInfo.RemoteName, dgst, auth)
				if err != nil {
					out.Write(sf.FormatProgress(stringid.TruncateID(layer.ID), "Image push failed", nil))
					return err
				}
			}
			if !exists {
				if pushDigest, err := s.pushV2Image(r, layer, endpoint, repoInfo.RemoteName, sf, out, auth); err != nil {
					return err
				} else if pushDigest != dgst {
					// Cache new checksum
					if err := s.graph.SetDigest(layer.ID, pushDigest); err != nil {
						return err
					}
					dgst = pushDigest
				}
			} else {
				out.Write(sf.FormatProgress(stringid.TruncateID(layer.ID), "Image already exists", nil))
			}
			m.FSLayers[i] = &registry.FSLayer{BlobSum: dgst.String()}
			m.History[i] = &registry.ManifestHistory{V1Compatibility: string(jsonData)}
		}

		if err := validateManifest(m); err != nil {
			return fmt.Errorf("invalid manifest: %s", err)
		}

		logrus.Debugf("Pushing %s:%s to v2 repository", repoInfo.LocalName, tag)
		mBytes, err := json.MarshalIndent(m, "", "   ")
		if err != nil {
			return err
		}
		js, err := libtrust.NewJSONSignature(mBytes)
		if err != nil {
			return err
		}

		if err = js.Sign(s.trustKey); err != nil {
			return err
		}

		signedBody, err := js.PrettySignature("signatures")
		if err != nil {
			return err
		}
		logrus.Infof("Signed manifest for %s:%s using daemon's key: %s", repoInfo.LocalName, tag, s.trustKey.KeyID())

		// push the manifest
		digest, err := r.PutV2ImageManifest(endpoint, repoInfo.RemoteName, tag, signedBody, mBytes, auth)
		if err != nil {
			return err
		}

		out.Write(sf.FormatStatus("", "Digest: %s", digest))
	}
	return nil
}

// PushV2Image pushes the image content to the v2 registry, first buffering the contents to disk
func (s *TagStore) pushV2Image(r *registry.Session, img *image.Image, endpoint *registry.Endpoint, imageName string, sf *streamformatter.StreamFormatter, out io.Writer, auth *registry.RequestAuthorization) (digest.Digest, error) {
	out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Buffering to Disk", nil))

	image, err := s.graph.Get(img.ID)
	if err != nil {
		return "", err
	}
	arch, err := s.graph.TarLayer(image)
	if err != nil {
		return "", err
	}
	defer arch.Close()

	tf, err := s.graph.newTempFile()
	if err != nil {
		return "", err
	}
	defer func() {
		tf.Close()
		os.Remove(tf.Name())
	}()

	size, dgst, err := bufferToFile(tf, arch)

	// Send the layer
	logrus.Debugf("rendered layer for %s of [%d] size", img.ID, size)

	if err := r.PutV2ImageBlob(endpoint, imageName, dgst,
		progressreader.New(progressreader.Config{
			In:        tf,
			Out:       out,
			Formatter: sf,
			Size:      int(size),
			NewLines:  false,
			ID:        stringid.TruncateID(img.ID),
			Action:    "Pushing",
		}), auth); err != nil {
		out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Image push failed", nil))
		return "", err
	}
	out.Write(sf.FormatProgress(stringid.TruncateID(img.ID), "Image successfully pushed", nil))
	return dgst, nil
}

// FIXME: Allow to interrupt current push when new push of same image is done.
func (s *TagStore) Push(localName string, imagePushConfig *ImagePushConfig) error {
	var (
		sf = streamformatter.NewJSONStreamFormatter()
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(localName)
	if err != nil {
		return err
	}

	if _, err := s.poolAdd("push", repoInfo.LocalName); err != nil {
		return err
	}
	defer s.poolRemove("push", repoInfo.LocalName)

	endpoint, err := repoInfo.GetEndpoint(imagePushConfig.MetaHeaders)
	if err != nil {
		return err
	}
	// TODO(tiborvass): reuse client from endpoint?
	// Adds Docker-specific headers as well as user-specified headers (metaHeaders)
	tr := transport.NewTransport(
		registry.NewTransport(registry.NoTimeout, endpoint.IsSecure),
		registry.DockerHeaders(imagePushConfig.MetaHeaders)...,
	)
	client := registry.HTTPClient(tr)
	r, err := registry.NewSession(client, imagePushConfig.AuthConfig, endpoint)
	if err != nil {
		return err
	}

	reposLen := 1
	if imagePushConfig.Tag == "" {
		reposLen = len(s.Repositories[repoInfo.LocalName])
	}

	imagePushConfig.OutStream.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", repoInfo.CanonicalName, reposLen))

	// If it fails, try to get the repository
	localRepo, exists := s.Repositories[repoInfo.LocalName]
	if !exists {
		return fmt.Errorf("Repository does not exist: %s", repoInfo.LocalName)
	}

	if repoInfo.Index.Official || endpoint.Version == registry.APIVersion2 {
		err := s.pushV2Repository(r, localRepo, imagePushConfig.OutStream, repoInfo, imagePushConfig.Tag, sf)
		if err == nil {
			s.eventsService.Log("push", repoInfo.LocalName, "")
			return nil
		}

		if err != ErrV2RegistryUnavailable {
			return fmt.Errorf("Error pushing to registry: %s", err)
		}
	}

	if err := s.pushRepository(r, imagePushConfig.OutStream, repoInfo, localRepo, imagePushConfig.Tag, sf); err != nil {
		return err
	}
	s.eventsService.Log("push", repoInfo.LocalName, "")
	return nil

}
