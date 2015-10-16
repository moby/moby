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

// byCreated is a temporary type used to sort a list of images by creation
// time.
type byCreated []*types.Image

func (r byCreated) Len() int           { return len(r) }
func (r byCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

// Images returns a filtered list of images. filterArgs is a JSON-encoded set
// of filter arguments which will be interpreted by pkg/parsers/filters.
// filter is a shell glob string applied to repository names. The argument
// named all controls whether all images in the graph are filtered, or just
// the heads.
func (s *TagStore) Images(filterArgs, filter string, all bool) ([]*types.Image, error) {
	var (
		allImages  map[string]*image.Image
		err        error
		filtTagged = true
		filtLabel  = false
	)

	imageFilters, err := filters.FromParam(filterArgs)
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
			if v := strings.ToLower(value); v == "true" {
				filtTagged = false
			} else if v != "false" {
				return nil, fmt.Errorf("Invalid filter 'dangling=%s'", v)
			}
		}
	}

	_, filtLabel = imageFilters["label"]

	if all && filtTagged {
		allImages = s.graph.Map()
	} else {
		allImages = s.graph.Heads()
	}

	lookup := make(map[string]*types.Image)
	s.Lock()
	for repoName, repository := range s.Repositories {
		filterTagName := ""
		if filter != "" {
			filterName := filter
			// Test if the tag was in there, if yes, get the name
			if strings.Contains(filterName, ":") {
				filterWithTag := strings.Split(filter, ":")
				filterName = filterWithTag[0]
				filterTagName = filterWithTag[1]
			}
			if match, _ := path.Match(filterName, repoName); !match {
				continue
			}
			if filterTagName != "" {
				if _, ok := repository[filterTagName]; !ok {
					continue
				}
			}
		}
		for ref, id := range repository {
			imgRef := utils.ImageReference(repoName, ref)
			if !strings.Contains(imgRef, filterTagName) {
				continue
			}
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

				if len(imageFilters["label"]) > 0 {
					if image.Config == nil {
						// Very old image that do not have image.Config (or even labels)
						continue
					}
					// We are now sure image.Config is not nil
					if !imageFilters.MatchKVList("label", image.Config.Labels) {
						continue
					}
				}
				if filtTagged {
					newImage := newImage(image, s.graph.GetParentsSize(image))

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
	if filter == "" || filtLabel {
		for _, image := range allImages {
			if len(imageFilters["label"]) > 0 {
				if image.Config == nil {
					// Very old image that do not have image.Config (or even labels)
					continue
				}
				// We are now sure image.Config is not nil
				if !imageFilters.MatchKVList("label", image.Config.Labels) {
					continue
				}
			}
			newImage := newImage(image, s.graph.GetParentsSize(image))
			newImage.RepoTags = []string{"<none>:<none>"}
			newImage.RepoDigests = []string{"<none>@<none>"}

			images = append(images, newImage)
		}
	}

	sort.Sort(sort.Reverse(byCreated(images)))

	return images, nil
}

func newImage(image *image.Image, parentSize int64) *types.Image {
	newImage := new(types.Image)
	newImage.ParentID = image.Parent
	newImage.ID = image.ID
	newImage.Created = image.Created.Unix()
	newImage.Size = image.Size
	newImage.VirtualSize = parentSize + image.Size
	if image.Config != nil {
		newImage.Labels = image.Config.Labels
	}
	return newImage
}
