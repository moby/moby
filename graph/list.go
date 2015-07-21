package graph

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/utils"
)

var acceptedImageFilterTags = map[string]struct{}{
	"dangling": {},
	"label":    {},
}

// ImagesConfig defines the criteria to obtain a list of images.
type ImagesConfig struct {
	// Filters is supported list of filters used to get list of images.
	Filters string
	// Filter the list of images by name.
	Filter string
	// All inditest that all the images will be returned in the list, if set to true.
	All bool
}

// byCreated is a temporary type used to sort list of images on their field 'Created'.
type byCreated []*types.Image

func (r byCreated) Len() int           { return len(r) }
func (r byCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

// Images provide list of images based on selection criteria.
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
		allImages = s.graph.Map()
	} else {
		allImages = s.graph.Heads()
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
				logrus.Warnf("couldn't load %s from %s: %s", id, imgRef, err)
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

	sort.Sort(sort.Reverse(byCreated(images)))

	return images, nil
}
