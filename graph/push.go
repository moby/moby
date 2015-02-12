package graph

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"github.com/docker/libtrust"
)

var ErrV2RegistryUnavailable = errors.New("error v2 registry unavailable")

// Retrieve the all the images to be uploaded in the correct order
func (s *TagStore) getImageList(localRepo map[string]string, requestedTag string) ([]string, map[string][]string, error) {
	var (
		imageList   []string
		imagesSeen  = make(map[string]bool)
		tagsByImage = make(map[string][]string)
	)

	for tag, id := range localRepo {
		if requestedTag != "" && requestedTag != tag {
			continue
		}
		var imageListForThisTag []string

		tagsByImage[id] = append(tagsByImage[id], tag)

		for img, err := s.graph.Get(id); img != nil; img, err = img.GetParent() {
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
	log.Debugf("Image list: %v", imageList)
	log.Debugf("Tags by image: %v", tagsByImage)

	return imageList, tagsByImage, nil
}

func (s *TagStore) getImageTags(localName, askedTag string) ([]string, error) {
	localRepo, err := s.Get(localName)
	if err != nil {
		return nil, err
	}
	log.Debugf("Checking %s against %#v", askedTag, localRepo)
	if len(askedTag) > 0 {
		if _, ok := localRepo[askedTag]; !ok {
			return nil, fmt.Errorf("Tag does not exist for %s:%s", localName, askedTag)
		}
		return []string{askedTag}, nil
	}
	var tags []string
	for tag := range localRepo {
		tags = append(tags, tag)
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
func lookupImageOnEndpoint(wg *sync.WaitGroup, r *registry.Session, out io.Writer, sf *utils.StreamFormatter,
	images chan imagePushData, imagesToPush chan string) {
	defer wg.Done()
	for image := range images {
		if err := r.LookupRemoteImage(image.id, image.endpoint, image.tokens); err != nil {
			log.Errorf("Error in LookupRemoteImage: %s", err)
			imagesToPush <- image.id
			continue
		}
		out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", utils.TruncateID(image.id)))
	}
}

func (s *TagStore) pushImageToEndpoint(endpoint string, out io.Writer, remoteName string, imageIDs []string,
	tags map[string][]string, repo *registry.RepositoryData, sf *utils.StreamFormatter, r *registry.Session) error {
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
	// is very important that is why we are still itterating over the ordered list of imageIDs.
	for _, id := range imageIDs {
		if _, push := shouldPush[id]; push {
			if _, err := s.pushImage(r, out, id, endpoint, repo.Tokens, sf); err != nil {
				// FIXME: Continue on error?
				return err
			}
		}
		for _, tag := range tags[id] {
			out.Write(sf.FormatStatus("", "Pushing tag for rev [%s] on {%s}", utils.TruncateID(id), endpoint+"repositories/"+remoteName+"/tags/"+tag))
			if err := r.PushRegistryTag(remoteName, id, tag, endpoint, repo.Tokens); err != nil {
				return err
			}
		}
	}
	return nil
}

// pushRepository pushes layers that do not already exist on the registry.
func (s *TagStore) pushRepository(r *registry.Session, out io.Writer,
	repoInfo *registry.RepositoryInfo, localRepo map[string]string,
	tag string, sf *utils.StreamFormatter) error {
	log.Debugf("Local repo: %s", localRepo)
	out = utils.NewWriteFlusher(out)
	imgList, tags, err := s.getImageList(localRepo, tag)
	if err != nil {
		return err
	}
	out.Write(sf.FormatStatus("", "Sending image list"))

	imageIndex := s.createImageIndex(imgList, tags)
	log.Debugf("Preparing to push %s with the following images and tags", localRepo)
	for _, data := range imageIndex {
		log.Debugf("Pushing ID: %s with Tag: %s", data.ID, data.Tag)
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

func (s *TagStore) pushImage(r *registry.Session, out io.Writer, imgID, ep string, token []string, sf *utils.StreamFormatter) (checksum string, err error) {
	out = utils.NewWriteFlusher(out)
	jsonRaw, err := ioutil.ReadFile(path.Join(s.graph.Root, imgID, "json"))
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve the path for {%s}: %s", imgID, err)
	}
	out.Write(sf.FormatProgress(utils.TruncateID(imgID), "Pushing", nil))

	imgData := &registry.ImgData{
		ID: imgID,
	}

	// Send the json
	if err := r.PushImageJSONRegistry(imgData, jsonRaw, ep, token); err != nil {
		if err == registry.ErrAlreadyExists {
			out.Write(sf.FormatProgress(utils.TruncateID(imgData.ID), "Image already pushed, skipping", nil))
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
	log.Debugf("rendered layer for %s of [%d] size", imgData.ID, layerData.Size)

	checksum, checksumPayload, err := r.PushImageLayerRegistry(imgData.ID, utils.ProgressReader(layerData, int(layerData.Size), out, sf, false, utils.TruncateID(imgData.ID), "Pushing"), ep, token, jsonRaw)
	if err != nil {
		return "", err
	}
	imgData.Checksum = checksum
	imgData.ChecksumPayload = checksumPayload
	// Send the checksum
	if err := r.PushImageChecksumRegistry(imgData, ep, token); err != nil {
		return "", err
	}

	out.Write(sf.FormatProgress(utils.TruncateID(imgData.ID), "Image successfully pushed", nil))
	return imgData.Checksum, nil
}

func (s *TagStore) pushV2Repository(r *registry.Session, eng *engine.Engine, out io.Writer, repoInfo *registry.RepositoryInfo, tag string, sf *utils.StreamFormatter) error {
	if repoInfo.Official {
		j := eng.Job("trust_update_base")
		if err := j.Run(); err != nil {
			log.Errorf("error updating trust base graph: %s", err)
		}
	}

	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			log.Infof("Unable to push to V2 registry, falling back to v1: %s", err)
			return ErrV2RegistryUnavailable
		}
		return fmt.Errorf("error getting registry endpoint: %s", err)
	}

	tags, err := s.getImageTags(repoInfo.LocalName, tag)
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
		log.Debugf("Pushing %s:%s to v2 repository", repoInfo.LocalName, tag)
		mBytes, err := s.newManifest(repoInfo.LocalName, repoInfo.RemoteName, tag)
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
		log.Infof("Signed manifest for %s:%s using daemon's key: %s", repoInfo.LocalName, tag, s.trustKey.KeyID())

		manifestBytes := string(signedBody)

		manifest, verified, err := s.loadManifest(eng, signedBody)
		if err != nil {
			return fmt.Errorf("error verifying manifest: %s", err)
		}

		if err := checkValidManifest(manifest); err != nil {
			return fmt.Errorf("invalid manifest: %s", err)
		}

		if verified {
			log.Infof("Pushing verified image, key %s is registered for %q", s.trustKey.KeyID(), repoInfo.RemoteName)
		}

		for i := len(manifest.FSLayers) - 1; i >= 0; i-- {
			var (
				sumStr  = manifest.FSLayers[i].BlobSum
				imgJSON = []byte(manifest.History[i].V1Compatibility)
			)

			sumParts := strings.SplitN(sumStr, ":", 2)
			if len(sumParts) < 2 {
				return fmt.Errorf("Invalid checksum: %s", sumStr)
			}
			manifestSum := sumParts[1]

			img, err := image.NewImgJSON(imgJSON)
			if err != nil {
				return fmt.Errorf("Failed to parse json: %s", err)
			}

			// Call mount blob
			exists, err := r.HeadV2ImageBlob(endpoint, repoInfo.RemoteName, sumParts[0], manifestSum, auth)
			if err != nil {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Image push failed", nil))
				return err
			}

			if !exists {
				if err := s.pushV2Image(r, img, endpoint, repoInfo.RemoteName, sumParts[0], manifestSum, sf, out, auth); err != nil {
					return err
				}
			} else {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Image already exists", nil))
			}
		}

		// push the manifest
		if err := r.PutV2ImageManifest(endpoint, repoInfo.RemoteName, tag, bytes.NewReader([]byte(manifestBytes)), auth); err != nil {
			return err
		}
	}
	return nil
}

// PushV2Image pushes the image content to the v2 registry, first buffering the contents to disk
func (s *TagStore) pushV2Image(r *registry.Session, img *image.Image, endpoint *registry.Endpoint, imageName, sumType, sumStr string, sf *utils.StreamFormatter, out io.Writer, auth *registry.RequestAuthorization) error {
	out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Buffering to Disk", nil))

	image, err := s.graph.Get(img.ID)
	if err != nil {
		return err
	}
	arch, err := image.TarLayer()
	if err != nil {
		return err
	}
	tf, err := s.graph.newTempFile()
	if err != nil {
		return err
	}
	defer func() {
		tf.Close()
		os.Remove(tf.Name())
	}()

	size, err := bufferToFile(tf, arch)
	if err != nil {
		return err
	}

	// Send the layer
	log.Debugf("rendered layer for %s of [%d] size", img.ID, size)

	if err := r.PutV2ImageBlob(endpoint, imageName, sumType, sumStr, utils.ProgressReader(tf, int(size), out, sf, false, utils.TruncateID(img.ID), "Pushing"), auth); err != nil {
		out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Image push failed", nil))
		return err
	}
	out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Image successfully pushed", nil))
	return nil
}

// FIXME: Allow to interrupt current push when new push of same image is done.
func (s *TagStore) CmdPush(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	var (
		localName   = job.Args[0]
		sf          = utils.NewStreamFormatter(job.GetenvBool("json"))
		authConfig  = &registry.AuthConfig{}
		metaHeaders map[string][]string
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ResolveRepositoryInfo(job, localName)
	if err != nil {
		return job.Error(err)
	}

	tag := job.Getenv("tag")
	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", &metaHeaders)

	if _, err := s.poolAdd("push", repoInfo.LocalName); err != nil {
		return job.Error(err)
	}
	defer s.poolRemove("push", repoInfo.LocalName)

	endpoint, err := repoInfo.GetEndpoint()
	if err != nil {
		return job.Error(err)
	}

	img, err := s.graph.Get(repoInfo.LocalName)
	r, err2 := registry.NewSession(authConfig, registry.HTTPRequestFactory(metaHeaders), endpoint, false)
	if err2 != nil {
		return job.Error(err2)
	}

	if endpoint.Version == registry.APIVersion2 {
		err := s.pushV2Repository(r, job.Eng, job.Stdout, repoInfo, tag, sf)
		if err == nil {
			return engine.StatusOK
		}

		if err != ErrV2RegistryUnavailable {
			return job.Errorf("Error pushing to registry: %s", err)
		}
	}

	if err != nil {
		reposLen := 1
		if tag == "" {
			reposLen = len(s.Repositories[repoInfo.LocalName])
		}
		job.Stdout.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", repoInfo.CanonicalName, reposLen))
		// If it fails, try to get the repository
		if localRepo, exists := s.Repositories[repoInfo.LocalName]; exists {
			if err := s.pushRepository(r, job.Stdout, repoInfo, localRepo, tag, sf); err != nil {
				return job.Error(err)
			}
			return engine.StatusOK
		}
		return job.Error(err)
	}

	var token []string
	job.Stdout.Write(sf.FormatStatus("", "The push refers to an image: [%s]", repoInfo.CanonicalName))
	if _, err := s.pushImage(r, job.Stdout, img.ID, endpoint.String(), token, sf); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
