package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/system"
)

var acceptedImageFilterTags = map[string]bool{
	"dangling":  true,
	"label":     true,
	"before":    true,
	"since":     true,
	"reference": true,
}

// byCreated is a temporary type used to sort a list of images by creation
// time.
type byCreated []*types.ImageSummary

func (r byCreated) Len() int           { return len(r) }
func (r byCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

// Images returns a filtered list of images. filterArgs is a JSON-encoded set
// of filter arguments which will be interpreted by api/types/filters.
// filter is a shell glob string applied to repository names. The argument
// named all controls whether all images in the graph are filtered, or just
// the heads.
func (i *ImageService) Images(ctx context.Context, imageFilters filters.Args, all bool, withExtraAttrs bool) ([]*types.ImageSummary, error) {
	if err := imageFilters.Validate(acceptedImageFilterTags); err != nil {
		return nil, err
	}

	danglingOnly := false
	if imageFilters.Contains("dangling") {
		if imageFilters.ExactMatch("dangling", "true") {
			danglingOnly = true
		} else if !imageFilters.ExactMatch("dangling", "false") {
			return nil, invalidFilter{"dangling", imageFilters.Get("dangling")}
		}
	}
	cs := i.client.ContentStore()

	var beforeFilter, sinceFilter *time.Time
	err := imageFilters.WalkValues("before", func(value string) error {
		img, err := i.ResolveImage(ctx, value)
		if err != nil {
			return err
		}
		info, err := cs.Info(ctx, img.Digest)
		if err != nil {
			return err
		}

		beforeFilter = &info.CreatedAt
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = imageFilters.WalkValues("since", func(value string) error {
		img, err := i.ResolveImage(ctx, value)
		if err != nil {
			return err
		}
		info, err := cs.Info(ctx, img.Digest)
		if err != nil {
			return err
		}

		sinceFilter = &info.CreatedAt
		return nil
	})
	if err != nil {
		return nil, err
	}

	var filters []string
	if danglingOnly {
		filters = append(filters, "name~=/sha256:[a-z0-9]+/")
	} else if imageFilters.Contains("reference") {
		for _, v := range imageFilters.Get("reference") {
			// TODO(containerd): Parse reference, if only partial match then
			// use as regex
			filters = append(filters, "name=="+v)
		}
	}

	if imageFilters.Contains("label") {
		var labels []string
		for _, v := range imageFilters.Get("label") {
			sv := strings.SplitN(v, "=", 2)
			if len(sv) == 2 {
				filters = append(filters, fmt.Sprintf("labels.%q==%s", sv[0], sv[1]))
			} else {
				filters = append(filters, "labels."+sv[0])
			}
		}

		labelFilter := strings.Join(labels, ",")

		if len(filters) == 0 {
			filters = append(filters, labelFilter)
		} else {
			for i := range filters {
				filters[i] = filters[i] + "," + labelFilter
			}
		}
	}

	allImages, err := i.client.ImageService().List(ctx, filters...)
	if err != nil {
		return nil, err
	}

	m := map[digest.Digest][]images.Image{}
	created := map[digest.Digest]time.Time{}

	for _, img := range allImages {
		info, err := cs.Info(ctx, img.Target.Digest)
		if err != nil {
			log.G(ctx).WithError(err).WithField("name", img.Name).Warnf("failed to stat target")
			continue
		}

		if beforeFilter != nil && !info.CreatedAt.Before(*beforeFilter) {
			continue
		}

		if sinceFilter != nil && !info.CreatedAt.After(*sinceFilter) {
			continue
		}

		m[img.Target.Digest] = append(m[img.Target.Digest], img)
		created[img.Target.Digest] = info.CreatedAt

		//var size int64
		// TODO: this seems pretty dumb to do
		//   Maybe we resolve a config and add size as a config label
		//layerID := img.RootFS.ChainID()
		//if layerID != "" {
		//	l, err := i.layerStores[img.OperatingSystem()].Get(layerID)
		//	if err != nil {
		//		// The layer may have been deleted between the call to `Map()` or
		//		// `Heads()` and the call to `Get()`, so we just ignore this error
		//		if err == layer.ErrLayerDoesNotExist {
		//			continue
		//		}
		//		return nil, err
		//	}

		//	size, err = l.Size()
		//	layer.ReleaseAndLog(i.layerStores[img.OperatingSystem()], l)
		//	if err != nil {
		//		return nil, err
		//	}
		//}

		//newImage := newImage(img, size)

		// TODO: Resolve config blob to get extra metadata
		// TODO: Store by target
		// TODO: Defer creation of image summary

		//if withExtraAttrs {
		//	// lazily init variables
		//	if imagesMap == nil {
		//		allContainers = i.containers.List()

		//		// allLayers is built from all layerstores combined
		//		allLayers = make(map[layer.ChainID]layer.Layer)
		//		for _, ls := range i.layerStores {
		//			layers := ls.Map()
		//			for k, v := range layers {
		//				allLayers[k] = v
		//			}
		//		}
		//		imagesMap = make(map[*image.Image]*types.ImageSummary)
		//		layerRefs = make(map[layer.ChainID]int)
		//	}

		//	// Get container count
		//	newImage.Containers = 0
		//	for _, c := range allContainers {
		//		if c.ImageID == id {
		//			newImage.Containers++
		//		}
		//	}

		//	// count layer references
		//	rootFS := *img.RootFS
		//	rootFS.DiffIDs = nil
		//	for _, id := range img.RootFS.DiffIDs {
		//		rootFS.Append(id)
		//		chid := rootFS.ChainID()
		//		layerRefs[chid]++
		//		if _, ok := allLayers[chid]; !ok {
		//			return nil, fmt.Errorf("layer %v was not found (corruption?)", chid)
		//		}
		//	}
		//	imagesMap[img] = newImage
		//}

		//images = append(images, newImage)
	}

	imageSums := []*types.ImageSummary{}
	//var layerRefs map[layer.ChainID]int
	//var allLayers map[layer.ChainID]layer.Layer
	//var allContainers []*container.Container

	for dgst, imgs := range m {
		newImage := new(types.ImageSummary)
		newImage.ID = dgst.String()
		newImage.Created = created[dgst].Unix()

		var target = imgs[0].Target
		var config ocispec.Descriptor

		switch target.MediaType {
		case images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
			config = target
		default:
			// TODO(containerd): use global platforms matcher
			platform := platforms.Default()

			// TODO(containerd): config matcher which ignores NotFound items?
			desc, err := images.Config(ctx, cs, imgs[0].Target, platform)
			if err != nil {
				log.G(ctx).WithError(err).WithField("image", dgst.String()).Warnf("unable to resolve config")
				continue
			}
			config = desc
		}

		// TODO(containerd): Stat config
		if info, err := cs.Info(ctx, config.Digest); err == nil {
			for label, value := range info.Labels {
				if label == LabelImageParent {
					newImage.ParentID = value
				} else if strings.HasPrefix(label, LabelLayerPrefix) {
					// TODO: Lookup from layer store
				}
				// TODO(containerd): Store size in label
			}
			// TODO(containerd): Resolve config for current platform
			// TODO(containerd): Fill this in from config and content labels
			//newImage.Size = size
			//newImage.VirtualSize = size
			//newImage.SharedSize = -1
			//newImage.Containers = -1
			//if image.Config != nil {
			//	newImage.Labels = image.Config.Labels
			//}
		} else {
			log.G(ctx).WithError(err).WithField("digest", config.Digest.String()).Warnf("unable to get image config info")
		}

		// TODO: Add each image reference
		// For these, unique them by manifest, none:none or none@digest
		digests := map[string]struct{}{}
		tags := map[string]struct{}{}

		for _, img := range imgs {
			ref, err := reference.Parse(img.Name)
			if err != nil {
				continue
			}
			if named, ok := ref.(reference.Named); ok {
				if c, ok := named.(reference.Canonical); ok {
					digests[reference.FamiliarString(c)] = struct{}{}
				} else if t, ok := named.(reference.Tagged); ok {
					tags[reference.FamiliarString(t)] = struct{}{}
				}
			}
		}

		for d := range digests {
			newImage.RepoDigests = append(newImage.RepoDigests, d)
		}
		for t := range tags {
			newImage.RepoTags = append(newImage.RepoTags, t)
		}

		if len(newImage.RepoTags) == 0 {
			// TODO(containerd): also skip if has children
			if !all {
				continue
			}

			if imageFilters.Contains("dangling") && !danglingOnly {
				//dangling=false case, so dangling image is not needed
				continue
			}

			if imageFilters.Contains("reference") { // skip images with no references if filtering by reference
				continue
			}

			if len(newImage.RepoDigests) == 0 {
				// TODO(containerd): Requires querying content store directly,
				// not currently possible
				newImage.RepoTags = []string{"none@none"}
			}
			newImage.RepoTags = []string{"none:none"}
		}

		imageSums = append(imageSums, newImage)
	}

	//if withExtraAttrs {
	//	// Get Shared sizes
	//	for img, newImage := range imagesMap {
	//		rootFS := *img.RootFS
	//		rootFS.DiffIDs = nil

	//		newImage.SharedSize = 0
	//		for _, id := range img.RootFS.DiffIDs {
	//			rootFS.Append(id)
	//			chid := rootFS.ChainID()

	//			diffSize, err := allLayers[chid].DiffSize()
	//			if err != nil {
	//				return nil, err
	//			}

	//			if layerRefs[chid] > 1 {
	//				newImage.SharedSize += diffSize
	//			}
	//		}
	//	}
	//}

	sort.Sort(sort.Reverse(byCreated(imageSums)))

	return imageSums, nil
}

// SquashImage creates a new image with the diff of the specified image and the specified parent.
// This new image contains only the layers from it's parent + 1 extra layer which contains the diff of all the layers in between.
// The existing image(s) is not destroyed.
// If no parent is specified, a new image with the diff of all the specified image's layers merged into a new layer that has no parents.
func (i *ImageService) SquashImage(id, parent string) (string, error) {

	var (
		img *image.Image
		err error
	)
	if img, err = i.imageStore.Get(image.ID(id)); err != nil {
		return "", err
	}

	var parentImg *image.Image
	var parentChainID layer.ChainID
	if len(parent) != 0 {
		parentImg, err = i.imageStore.Get(image.ID(parent))
		if err != nil {
			return "", errors.Wrap(err, "error getting specified parent layer")
		}
		parentChainID = parentImg.RootFS.ChainID()
	} else {
		rootFS := image.NewRootFS()
		parentImg = &image.Image{RootFS: rootFS}
	}
	if !system.IsOSSupported(img.OperatingSystem()) {
		return "", errors.Wrap(err, system.ErrNotSupportedOperatingSystem.Error())
	}
	l, err := i.layerStores[img.OperatingSystem()].Get(img.RootFS.ChainID())
	if err != nil {
		return "", errors.Wrap(err, "error getting image layer")
	}
	defer i.layerStores[img.OperatingSystem()].Release(l)

	ts, err := l.TarStreamFrom(parentChainID)
	if err != nil {
		return "", errors.Wrapf(err, "error getting tar stream to parent")
	}
	defer ts.Close()

	newL, err := i.layerStores[img.OperatingSystem()].Register(ts, parentChainID)
	if err != nil {
		return "", errors.Wrap(err, "error registering layer")
	}
	defer i.layerStores[img.OperatingSystem()].Release(newL)

	newImage := *img
	newImage.RootFS = nil

	rootFS := *parentImg.RootFS
	rootFS.DiffIDs = append(rootFS.DiffIDs, newL.DiffID())
	newImage.RootFS = &rootFS

	for i, hi := range newImage.History {
		if i >= len(parentImg.History) {
			hi.EmptyLayer = true
		}
		newImage.History[i] = hi
	}

	now := time.Now()
	var historyComment string
	if len(parent) > 0 {
		historyComment = fmt.Sprintf("merge %s to %s", id, parent)
	} else {
		historyComment = fmt.Sprintf("create new from %s", id)
	}

	newImage.History = append(newImage.History, image.History{
		Created: now,
		Comment: historyComment,
	})
	newImage.Created = now

	b, err := json.Marshal(&newImage)
	if err != nil {
		return "", errors.Wrap(err, "error marshalling image config")
	}

	newImgID, err := i.imageStore.Create(b)
	if err != nil {
		return "", errors.Wrap(err, "error creating new image after squash")
	}
	return string(newImgID), nil
}

func newImage(image *image.Image, size int64) *types.ImageSummary {
	newImage := new(types.ImageSummary)
	newImage.ParentID = image.Parent.String()
	newImage.ID = image.ID().String()
	newImage.Created = image.Created.Unix()
	newImage.Size = size
	newImage.VirtualSize = size
	newImage.SharedSize = -1
	newImage.Containers = -1
	if image.V1Image.Config != nil {
		newImage.Labels = image.V1Image.Config.Labels
	}
	return newImage
}
