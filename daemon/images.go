package daemon

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/parsers/filters"
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

// Map returns a map of all images in the ImageStore
func (daemon *Daemon) Map() map[image.ID]*image.Image {
	return daemon.imageStore.Map()
}

// Images returns a filtered list of images. filterArgs is a JSON-encoded set
// of filter arguments which will be interpreted by pkg/parsers/filters.
// filter is a shell glob string applied to repository names. The argument
// named all controls whether all images in the graph are filtered, or just
// the heads.
func (daemon *Daemon) Images(filterArgs, filter string, all bool) ([]*types.Image, error) {
	var (
		allImages    map[image.ID]*image.Image
		err          error
		danglingOnly = false
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
				danglingOnly = true
			} else if v != "false" {
				return nil, fmt.Errorf("Invalid filter 'dangling=%s'", v)
			}
		}
	}

	if danglingOnly {
		allImages = daemon.imageStore.Heads()
	} else {
		allImages = daemon.imageStore.Map()
	}

	images := []*types.Image{}

	var filterTagged bool
	if filter != "" {
		filterRef, err := reference.Parse(filter)
		if err == nil { // parse error means wildcard repo
			if _, ok := filterRef.(reference.Tagged); ok {
				filterTagged = true
			}
		}
	}

	for id, img := range allImages {
		if _, ok := imageFilters["label"]; ok {
			if img.Config == nil {
				// Very old image that do not have image.Config (or even labels)
				continue
			}
			// We are now sure image.Config is not nil
			if !imageFilters.MatchKVList("label", img.Config.Labels) {
				continue
			}
		}

		layerID := img.RootFS.ChainID()
		var size int64
		if layerID != "" {
			l, err := daemon.layerStore.Get(layerID)
			if err != nil {
				return nil, err
			}

			size, err = l.Size()
			layer.ReleaseAndLog(daemon.layerStore, l)
			if err != nil {
				return nil, err
			}
		}

		newImage := newImage(img, size)

		for _, ref := range daemon.tagStore.References(id) {
			if filter != "" { // filter by tag/repo name
				if filterTagged { // filter by tag, require full ref match
					if ref.String() != filter {
						continue
					}
				} else if matched, err := path.Match(filter, ref.Name()); !matched || err != nil { // name only match, FIXME: docs say exact
					continue
				}
			}
			if _, ok := ref.(reference.Digested); ok {
				newImage.RepoDigests = append(newImage.RepoDigests, ref.String())
			}
			if _, ok := ref.(reference.Tagged); ok {
				newImage.RepoTags = append(newImage.RepoTags, ref.String())
			}
		}
		if newImage.RepoDigests == nil && newImage.RepoTags == nil {
			if all || len(daemon.imageStore.Children(id)) == 0 {
				if filter != "" { // skip images with no references if filtering by tag
					continue
				}
				newImage.RepoDigests = []string{"<none>@<none>"}
				newImage.RepoTags = []string{"<none>:<none>"}
			} else {
				continue
			}
		} else if danglingOnly {
			continue
		}

		images = append(images, newImage)
	}

	sort.Sort(sort.Reverse(byCreated(images)))

	return images, nil
}

func newImage(image *image.Image, size int64) *types.Image {
	newImage := new(types.Image)
	newImage.ParentID = image.Parent.String()
	newImage.ID = image.ID().String()
	newImage.Created = image.Created.Unix()
	newImage.Size = size
	newImage.VirtualSize = size
	if image.Config != nil {
		newImage.Labels = image.Config.Labels
	}
	return newImage
}
