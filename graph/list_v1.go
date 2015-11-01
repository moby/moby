package graph

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
)

type v1TagLister struct {
	*TagStore
	endpoint registry.APIEndpoint
	config   *RemoteTagsConfig
	repoInfo *registry.RepositoryInfo
	session  *registry.Session
}

func (p *v1TagLister) ListTags() ([]*types.RepositoryTag, bool, error) {
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
	tagList, err := p.listTagsWithSession()
	return tagList, false, err
}

func (p *v1TagLister) listTagsWithSession() ([]*types.RepositoryTag, error) {
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

	tagList := make([]*types.RepositoryTag, 0, len(tagsList))
	for tag, imageID := range tagsList {
		tagList = append(tagList, &types.RepositoryTag{Tag: tag, ImageID: imageID})
	}

	return tagList, nil
}
