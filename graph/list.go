package graph

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/utils"
)

var acceptedImageFilterTags = map[string]struct{}{
	"dangling": {},
	"label":    {},
}

type ByCreated []*types.Image

func (r ByCreated) Len() int           { return len(r) }
func (r ByCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ByCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

func (s *TagStore) CmdImages(job *engine.Job) error {
	var (
		allImages  map[string]*image.Image
		err        error
		filtTagged = true
		filtLabel  = false
	)

	imageFilters, err := filters.FromParam(job.Getenv("filters"))
	if err != nil {
		return err
	}
	for name := range imageFilters {
		if _, ok := acceptedImageFilterTags[name]; !ok {
			return fmt.Errorf("Invalid filter '%s'", name)
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

	if job.GetenvBool("all") && filtTagged {
		allImages, err = s.graph.Map()
	} else {
		allImages, err = s.graph.Heads()
	}
	if err != nil {
		return err
	}

	lookup := make(map[string]*types.Image)
	s.Lock()
	for repoName, repository := range s.Repositories {
		if job.Getenv("filter") != "" {
			if match, _ := path.Match(job.Getenv("filter"), repoName); !match {
				continue
			}
		}
		for ref, id := range repository {
			imgRef := utils.ImageReference(repoName, ref)
			image, err := s.graph.Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s: %s", id, imgRef, err)
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
					newImage.VirtualSize = int(image.GetParentsSize(0) + image.Size)
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
	if job.Getenv("filter") == "" || filtLabel {
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
			newImage.VirtualSize = int(image.GetParentsSize(0) + image.Size)
			newImage.Labels = image.ContainerConfig.Labels

			images = append(images, newImage)
		}
	}

	sort.Sort(sort.Reverse(ByCreated(images)))

	if err = json.NewEncoder(job.Stdout).Encode(images); err != nil {
		return err
	}
	return nil
}
