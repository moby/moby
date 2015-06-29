package graph

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/transport"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

var acceptedImageFilterTags = map[string]struct{}{
	"dangling": {},
	"label":    {},
}

type ImagesConfig struct {
	Filters string
	Filter  string
	All     bool
}

type ByCreated []*types.Image

func (r ByCreated) Len() int           { return len(r) }
func (r ByCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ByCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

type ByTagName []*types.RepositoryTag

func (r ByTagName) Len() int           { return len(r) }
func (r ByTagName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ByTagName) Less(i, j int) bool { return r[i].Tag < r[j].Tag }

type TagsConfig struct {
	MetaHeaders map[string][]string
	AuthConfig  *cliconfig.AuthConfig
}

func (s *TagStore) Images(config *ImagesConfig) ([]*types.Image, error) {
	var (
		allImages  map[string]*image.Image
		err        error
		filtTagged = true
		filtLabel  = false
	)

	imageFilters, err := filters.FromParam(config.Filters)
	if err != nil {
		return nil, err
	}
	for name := range imageFilters {
		if _, ok := acceptedImageFilterTags[name]; !ok {
			return nil, fmt.Errorf("Invalid filter '%s'", name)
		}
	}

	if i, ok := imageFilters["dangling"]; ok {
		for _, value := range i {
			if strings.ToLower(value) == "true" {
				filtTagged = false
			}
		}
	}

	_, filtLabel = imageFilters["label"]

	if config.All && filtTagged {
		allImages, err = s.graph.Map()
	} else {
		allImages, err = s.graph.Heads()
	}
	if err != nil {
		return nil, err
	}

	lookup := make(map[string]*types.Image)
	s.Lock()
	for repoName, repository := range s.Repositories {
		if config.Filter != "" {
			if match, _ := path.Match(config.Filter, repoName); !match {
				continue
			}
		}
		for ref, id := range repository {
			imgRef := utils.ImageReference(repoName, ref)
			image, err := s.graph.Get(id)
			if err != nil {
				logrus.Printf("Warning: couldn't load %s from %s: %s", id, imgRef, err)
				continue
			}

			if lImage, exists := lookup[id]; exists {
				if filtTagged {
					if utils.DigestReference(ref) {
						lImage.RepoDigests = append(lImage.RepoDigests, imgRef)
					} else { // Tag Ref.
						lImage.RepoTags = append(lImage.RepoTags, imgRef)
					}
				}
			} else {
				// get the boolean list for if only the untagged images are requested
				delete(allImages, id)
				if !imageFilters.MatchKVList("label", image.ContainerConfig.Labels) {
					continue
				}
				if filtTagged {
					newImage := new(types.Image)
					newImage.ParentId = image.Parent
					newImage.ID = image.ID
					newImage.Created = int(image.Created.Unix())
					newImage.Size = int(image.Size)
					newImage.VirtualSize = int(s.graph.GetParentsSize(image, 0) + image.Size)
					newImage.Labels = image.ContainerConfig.Labels

					if utils.DigestReference(ref) {
						newImage.RepoTags = []string{}
						newImage.RepoDigests = []string{imgRef}
					} else {
						newImage.RepoTags = []string{imgRef}
						newImage.RepoDigests = []string{}
					}

					lookup[id] = newImage
				}
			}

		}
	}
	s.Unlock()

	images := []*types.Image{}
	for _, value := range lookup {
		images = append(images, value)
	}

	// Display images which aren't part of a repository/tag
	if config.Filter == "" || filtLabel {
		for _, image := range allImages {
			if !imageFilters.MatchKVList("label", image.ContainerConfig.Labels) {
				continue
			}
			newImage := new(types.Image)
			newImage.ParentId = image.Parent
			newImage.RepoTags = []string{"<none>:<none>"}
			newImage.RepoDigests = []string{"<none>@<none>"}
			newImage.ID = image.ID
			newImage.Created = int(image.Created.Unix())
			newImage.Size = int(image.Size)
			newImage.VirtualSize = int(s.graph.GetParentsSize(image, 0) + image.Size)
			newImage.Labels = image.ContainerConfig.Labels

			images = append(images, newImage)
		}
	}

	sort.Sort(sort.Reverse(ByCreated(images)))

	return images, nil
}

func (s *TagStore) Tags(name string, config *TagsConfig) (*types.RepositoryTagList, error) {
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

	tagList := &types.RepositoryTagList{}
	tagList.Name = repoInfo.CanonicalName

	// first try v1 endpoint because it gives us tags with associated IDs
	tagList.TagList, err = s.getRemoteV1TagList(r, repoInfo, config.AuthConfig)
	if err != nil && endpoint.Version == registry.APIVersion1 {
		return nil, err
	}
	if err == nil {
		sort.Sort(ByTagName(tagList.TagList))
		return tagList, err
	}
	tagList.TagList, err = s.getRemoteV2TagList(r, repoInfo)
	if err != nil {
		return nil, err
	}
	sort.Sort(ByTagName(tagList.TagList))
	return tagList, nil
}

func (s *TagStore) getRemoteV1TagList(r *registry.Session, repoInfo *registry.RepositoryInfo, auth *cliconfig.AuthConfig) ([]*types.RepositoryTag, error) {
	repoData, err := r.GetRepositoryData(repoInfo.RemoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return nil, fmt.Errorf("Error: repository %s not found", repoInfo.RemoteName)
		}
		// Unexpected HTTP error
		return nil, err
	}

	logrus.Debugf("Retrieving the tag list from V1 endpoints")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, repoInfo.RemoteName)
	if err != nil {
		logrus.Errorf("Unable to get remote tags: %s", err)
		return nil, err
	}
	if len(tagsList) < 1 {
		return nil, fmt.Errorf("No tags available for remote repository %s", repoInfo.CanonicalName)
	}

	tagList := make([]*types.RepositoryTag, 0, len(tagsList))
	for tag, imageID := range tagsList {
		tagList = append(tagList, &types.RepositoryTag{Tag: tag, ImageID: imageID})
	}

	return tagList, nil
}

func (s *TagStore) getRemoteV2TagList(r *registry.Session, repoInfo *registry.RepositoryInfo) ([]*types.RepositoryTag, error) {
	endpoint, err := r.V2RegistryEndpoint(repoInfo.Index)
	if err != nil {
		if repoInfo.Index.Official {
			logrus.Debugf("Unable to get remote tags from V2 registry: %v", err)
			return nil, ErrV2RegistryUnavailable
		}
		return nil, fmt.Errorf("error getting registry endpoint: %s", err)
	}
	auth, err := r.GetV2Authorization(endpoint, repoInfo.RemoteName, true)
	if err != nil {
		return nil, fmt.Errorf("error getting authorization: %s", err)
	}
	logrus.Debugf("Retrieving the tag list from V2 endpoint %v", endpoint.URL)
	tags, err := r.GetV2RemoteTags(endpoint, repoInfo.RemoteName, auth)
	if err != nil {
		return nil, fmt.Errorf("Failed to get remote tags: %v", err)
	}
	tagList := make([]*types.RepositoryTag, len(tags))
	for i, tag := range tags {
		tagList[i] = &types.RepositoryTag{Tag: tag}
	}
	return tagList, nil
}
