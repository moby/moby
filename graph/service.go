package graph

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/transport"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// LookupRemoteConfig allows you to pass transport-related data to LookupRemote
// function.
type LookupRemoteConfig struct {
	MetaHeaders map[string][]string
	AuthConfig  *cliconfig.AuthConfig
}

func (s *TagStore) LookupRaw(name string) ([]byte, error) {
	image, err := s.LookupImage(name)
	if err != nil || image == nil {
		return nil, fmt.Errorf("No such image %s", name)
	}

	imageInspectRaw, err := s.graph.RawJSON(image.ID)
	if err != nil {
		return nil, err
	}

	return imageInspectRaw, nil
}

// Lookup return an image encoded in JSON
func (s *TagStore) Lookup(name string) (*types.ImageInspect, error) {
	image, err := s.LookupImage(name)
	if err != nil || image == nil {
		return nil, fmt.Errorf("No such image: %s", name)
	}

	imageInspect := &types.ImageInspect{
		types.ImageInspectBase{
			Id:              image.ID,
			Parent:          image.Parent,
			Comment:         image.Comment,
			Created:         image.Created,
			Container:       image.Container,
			ContainerConfig: &image.ContainerConfig,
			DockerVersion:   image.DockerVersion,
			Author:          image.Author,
			Config:          image.Config,
			Architecture:    image.Architecture,
			Os:              image.OS,
			Size:            image.Size,
		},
		s.graph.GetParentsSize(image, 0) + image.Size,
		types.GraphDriverData{
			Name: s.graph.driver.String(),
		},
	}

	graphDriverData, err := s.graph.driver.GetMetadata(image.ID)
	if err != nil {
		return nil, err
	}
	imageInspect.GraphDriver.Data = graphDriverData

	return imageInspect, nil
}

func (s *TagStore) LookupRemote(name, tag string, config *LookupRemoteConfig) (*types.RemoteImageInspect, error) {
	var (
		img  *Image
		dgst digest.Digest
	)

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(name)
	if err != nil {
		return nil, err
	}

	if err := validateRepoName(repoInfo.LocalName); err != nil {
		return nil, err
	}

	endpoint, err := repoInfo.GetEndpoint(config.MetaHeaders)
	if err != nil {
		return nil, err
	}

	tr := transport.NewTransport(
		registry.NewTransport(registry.ReceiveTimeout, endpoint.IsSecure),
		registry.DockerHeaders(config.MetaHeaders)...,
	)
	client := registry.HTTPClient(tr)
	r, err := registry.NewSession(client, config.AuthConfig, endpoint)
	if err != nil {
		return nil, err
	}

	logName := repoInfo.CanonicalName
	if tag != "" {
		logName = utils.ImageReference(logName, tag)
	}

	if len(repoInfo.Index.Mirrors) == 0 && (repoInfo.Index.Official || endpoint.Version == registry.APIVersion2) {
		if repoInfo.Official {
			s.trustService.UpdateBase()
		}

		logrus.Debugf("pulling v2 repository manifest named %q", logName)
		if img, tag, dgst, err = s.pullJSONFromV2Registry(r, repoInfo, tag); err == nil {
			s.eventsService.Log("inspect", logName, "")
		} else if err != registry.ErrDoesNotExist && err != ErrV2RegistryUnavailable {
			logrus.Errorf("Error from V2 registry: %s", err)
		}
	}

	if img == nil {
		logrus.Debugf("pulling v1 repository manifest named %q", logName)
		if img, tag, err = s.pullJSONFromRegistry(r, repoInfo, tag); err != nil {
			return nil, err
		}
	}

	imageInspect := &types.RemoteImageInspect{
		types.ImageInspectBase{
			Id:              img.ID,
			Parent:          img.Parent,
			Comment:         img.Comment,
			Created:         img.Created,
			Container:       img.Container,
			ContainerConfig: &img.ContainerConfig,
			DockerVersion:   img.DockerVersion,
			Author:          img.Author,
			Config:          img.Config,
			Architecture:    img.Architecture,
			Os:              img.OS,
			Size:            img.Size,
		},
		repoInfo.Index.Name,
		dgst.String(),
		tag,
	}

	return imageInspect, nil
}

func (s *TagStore) pullJSONFromV2Registry(r *registry.Session, repoInfo *registry.RepositoryInfo, tag string) (*Image, string, digest.Digest, error) {
	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			logrus.Debugf("Unable to pull from V2 registry, falling back to v1: %s", err)
			return nil, tag, "", ErrV2RegistryUnavailable
		}
		return nil, tag, "", fmt.Errorf("error getting registry endpoint: %s", err)
	}
	auth, err := r.GetV2Authorization(endpoint, repoInfo.RemoteName, true)
	if err != nil {
		return nil, tag, "", fmt.Errorf("error getting authorization: %s", err)
	}
	if tag == "" {
		logrus.Debugf("Retrieving V2 tag list")
		tags, err := r.GetV2RemoteTags(endpoint, repoInfo.RemoteName, auth)
		if err != nil {
			return nil, tag, "", fmt.Errorf("Failed to get remote tags: %v", err)
		}
		for _, t := range tags {
			if t == DEFAULTTAG {
				tag = DEFAULTTAG
			}
		}
		if tag == "" && len(tags) > 0 {
			tag = tags[0]
		}
		if tag == "" {
			return nil, "", "", fmt.Errorf("No tags available for repository %s", repoInfo.CanonicalName)
		}
	}
	img, dgst, err := s.pullV2ImageJSON(r, endpoint, repoInfo, tag, auth)
	if err == nil && dgst.String() == tag {
		// Don't show digest as a tag
		tag = ""
	}
	return img, tag, dgst, err
}

func (s *TagStore) pullV2ImageJSON(r *registry.Session, endpoint *registry.Endpoint, repoInfo *registry.RepositoryInfo, tag string, auth *registry.RequestAuthorization) (*Image, digest.Digest, error) {
	var (
		err error
		img *Image
	)
	logrus.Debugf("Pulling tag from V2 registry: %q", tag)

	remoteDigest, manifestBytes, err := r.GetV2ImageManifest(endpoint, repoInfo.RemoteName, tag, auth)
	if err != nil {
		return nil, "", err
	}

	// loadManifest ensures that the manifest payload has the expected digest
	// if the tag is a digest reference.
	_, manifest, verified, err := s.loadManifest(manifestBytes, tag, remoteDigest)
	if err != nil {
		return nil, "", fmt.Errorf("error verifying manifest: %s", err)
	}

	if verified {
		logrus.Infof("Image manifest for %s has been verified", utils.ImageReference(repoInfo.CanonicalName, tag))
	}

	if len(manifest.FSLayers) < 1 {
		return nil, "", fmt.Errorf("No layer in obtained manifest!")
	}
	imgJSON := []byte(manifest.History[0].V1Compatibility)
	img, err = NewImgJSON(imgJSON)
	return img, remoteDigest, err
}

func (s *TagStore) pullJSONFromRegistry(r *registry.Session, repoInfo *registry.RepositoryInfo, askedTag string) (*Image, string, error) {
	repoData, err := r.GetRepositoryData(repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, "", fmt.Errorf("Error: image %s not found", utils.ImageReference(repoInfo.RemoteName, askedTag))
		}
		// Unexpected HTTP error
		return nil, "", err
	}

	logrus.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, repoInfo.RemoteName)
	if err != nil {
		logrus.Errorf("Unable to get remote tags: %s", err)
		return nil, "", err
	}
	if len(tagsList) < 1 {
		return nil, "", fmt.Errorf("No tags available for remote repository %s", repoInfo.CanonicalName)
	}

	for tag, id := range tagsList {
		repoData.ImgList[id] = &registry.ImgData{
			ID:       id,
			Tag:      tag,
			Checksum: "",
		}
	}

	// If no tag has been specified, choose `latest` if it exists
	if askedTag == "" {
		if _, exists := tagsList[DEFAULTTAG]; exists {
			askedTag = DEFAULTTAG
		}
	}
	if askedTag == "" {
		// fallback to any tag in the repository
		for tag := range tagsList {
			askedTag = tag
			break
		}
	}

	id, exists := tagsList[askedTag]
	if !exists {
		return nil, "", fmt.Errorf("Tag %s not found in repository %s", askedTag, repoInfo.CanonicalName)
	}
	img := repoData.ImgList[id]

	var pulledImg *Image
	for _, ep := range repoInfo.Index.Mirrors {
		if pulledImg, err = s.pullImageJSON(r, img.ID, ep, repoData.Tokens); err != nil {
			// Don't report errors when pulling from mirrors.
			logrus.Debugf("Error pulling image json of %s:%s, mirror: %s, %s", repoInfo.CanonicalName, img.Tag, ep, err)
			continue
		}
		break
	}
	if pulledImg == nil {
		for _, ep := range repoData.Endpoints {
			if pulledImg, err = s.pullImageJSON(r, img.ID, ep, repoData.Tokens); err != nil {
				// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
				logrus.Infof("Error pulling image json of %s:%s, endpoint: %s, %v", repoInfo.CanonicalName, img.Tag, ep, err)
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, repoInfo.CanonicalName, err)
	}
	if pulledImg == nil {
		return nil, "", fmt.Errorf("No such image %s:%s", repoInfo.CanonicalName, askedTag)
	}

	return pulledImg, askedTag, nil
}

func (s *TagStore) pullImageJSON(r *registry.Session, imgID, endpoint string, token []string) (*Image, error) {
	imgJSON, _, err := r.GetRemoteImageJSON(imgID, endpoint)
	if err != nil {
		return nil, err
	}
	img, err := NewImgJSON(imgJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse json: %s", err)
	}
	return img, nil
}

// ImageTarLayer return the tarLayer of the image
func (s *TagStore) ImageTarLayer(name string, dest io.Writer) error {
	if image, err := s.LookupImage(name); err == nil && image != nil {
		// On Windows, the base layer cannot be exported
		if runtime.GOOS != "windows" || image.Parent != "" {

			fs, err := s.graph.TarLayer(image)
			if err != nil {
				return err
			}
			defer fs.Close()

			written, err := io.Copy(dest, fs)
			if err != nil {
				return err
			}
			logrus.Debugf("rendered layer for %s of [%d] size", image.ID, written)
		}
		return nil
	}
	return fmt.Errorf("No such image: %s", name)
}
