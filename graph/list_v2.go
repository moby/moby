package graph

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"golang.org/x/net/context"
)

type v2TagLister struct {
	*TagStore
	endpoint registry.APIEndpoint
	config   *RemoteTagsConfig
	repoInfo *registry.RepositoryInfo
	repo     distribution.Repository
}

func (p *v2TagLister) ListTags() (tagList []*types.RepositoryTag, fallback bool, err error) {
	p.repo, err = NewV2Repository(p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig)
	if err != nil {
		logrus.Debugf("Error getting v2 registry: %v", err)
		return nil, true, err
	}

	tagList, err = p.listTagsWithRepository()
	if err != nil && registry.ContinueOnError(err) {
		logrus.Debugf("Error trying v2 registry: %v", err)
		fallback = true
	}
	return
}

func (p *v2TagLister) listTagsWithRepository() ([]*types.RepositoryTag, error) {
	logrus.Debugf("Retrieving the tag list from V2 endpoint %v", p.endpoint.URL)
	manSvc, err := p.repo.Manifests(context.Background())
	if err != nil {
		return nil, err
	}
	tags, err := manSvc.Tags()
	if err != nil {
		return nil, err
	}
	tagList := make([]*types.RepositoryTag, len(tags))
	for i, tag := range tags {
		tagList[i] = &types.RepositoryTag{Tag: tag}
	}
	return tagList, nil
}
