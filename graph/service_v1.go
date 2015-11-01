package graph

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

type v1ManifestFetcher struct {
	*TagStore
	endpoint registry.APIEndpoint
	config   *LookupRemoteConfig
	repoInfo *registry.RepositoryInfo
	session  *registry.Session
}

func (p *v1ManifestFetcher) Fetch(ref string) (imgInspect *types.RemoteImageInspect, fallback bool, err error) {
	if utils.DigestReference(ref) {
		// Allowing fallback, because HTTPS v1 is before HTTP v2
		return nil, true, registry.ErrNoSupport{errors.New("Cannot pull by digest with v1 registry")}
	}
	tlsConfig, err := p.registryService.TLSConfig(p.repoInfo.Index.Name)
	if err != nil {
		return nil, false, err
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
		return nil, true, err
	}
	p.session, err = registry.NewSession(client, p.config.AuthConfig, v1Endpoint)
	if err != nil {
		// TODO(dmcgowan): Check if should fallback
		logrus.Debugf("Fallback from error: %s", err)
		return nil, true, err
	}
	imgInspect, err = p.fetchWithSession(ref)
	return
}

func (p *v1ManifestFetcher) fetchWithSession(askedTag string) (*types.RemoteImageInspect, error) {
	repoData, err := p.session.GetRepositoryData(p.repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, fmt.Errorf("Error: image %s not found", p.repoInfo.RemoteName)
		}
		// Unexpected HTTP error
		return nil, err
	}

	logrus.Debugf("Retrieving the tag list from V1 endpoints")
	tagsList, err := p.session.GetRemoteTags(repoData.Endpoints, p.repoInfo.RemoteName)
	if err != nil {
		logrus.Errorf("Unable to get remote tags: %s", err)
		return nil, err
	}
	if len(tagsList) < 1 {
		return nil, fmt.Errorf("No tags available for remote repository %s", p.repoInfo.CanonicalName)
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
		if _, exists := tagsList[tags.DefaultTag]; exists {
			askedTag = tags.DefaultTag
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
		return nil, fmt.Errorf("Tag %s not found in repository %s", askedTag, p.repoInfo.CanonicalName)
	}
	img := repoData.ImgList[id]

	var pulledImg *image.Image
	for _, ep := range p.repoInfo.Index.Mirrors {
		if pulledImg, err = p.pullImageJSON(img.ID, ep, repoData.Tokens); err != nil {
			// Don't report errors when pulling from mirrors.
			logrus.Debugf("Error pulling image json of %s:%s, mirror: %s, %s", p.repoInfo.CanonicalName, img.Tag, ep, err)
			continue
		}
		break
	}
	if pulledImg == nil {
		for _, ep := range repoData.Endpoints {
			if pulledImg, err = p.pullImageJSON(img.ID, ep, repoData.Tokens); err != nil {
				// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
				logrus.Infof("Error pulling image json of %s:%s, endpoint: %s, %v", p.repoInfo.CanonicalName, img.Tag, ep, err)
				continue
			}
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, p.repoInfo.CanonicalName, err)
	}
	if pulledImg == nil {
		return nil, fmt.Errorf("No such image %s:%s", p.repoInfo.CanonicalName, askedTag)
	}

	return makeRemoteImageInspect(p.repoInfo, pulledImg, askedTag, ""), nil
}

func (p *v1ManifestFetcher) pullImageJSON(imgID, endpoint string, token []string) (*image.Image, error) {
	imgJSON, _, err := p.session.GetRemoteImageJSON(imgID, endpoint)
	if err != nil {
		return nil, err
	}
	img, err := image.NewImgJSON(imgJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse json: %s", err)
	}
	return img, nil
}
