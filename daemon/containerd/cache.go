package containerd

import (
	"context"
	"reflect"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, cacheFrom []string) (builder.ImageCache, error) {
	images := []*image.Image{}
	if len(cacheFrom) == 0 {
		return &localCache{
			imageService: i,
		}, nil
	}

	for _, c := range cacheFrom {
		h, err := i.ImageHistory(ctx, c)
		if err != nil {
			continue
		}
		for _, hi := range h {
			if hi.ID != "<missing>" {
				im, err := i.GetImage(ctx, hi.ID, imagetype.GetImageOpts{})
				if err != nil {
					return nil, err
				}
				images = append(images, im)
			}
		}
	}

	return &imageCache{
		lc: &localCache{
			imageService: i,
		},
		images:       images,
		imageService: i,
	}, nil
}

type localCache struct {
	imageService *ImageService
}

func (ic *localCache) GetCache(parentID string, cfg *container.Config) (imageID string, err error) {
	ctx := context.TODO()

	var children []image.ID

	// FROM scratch
	if parentID == "" {
		imgs, err := ic.imageService.Images(ctx, types.ImageListOptions{
			All: true,
		})
		if err != nil {
			return "", err
		}
		for _, img := range imgs {
			if img.ParentID == parentID {
				children = append(children, image.ID(img.ID))
			}
		}
	} else {
		c, err := ic.imageService.Children(ctx, image.ID(parentID))
		if err != nil {
			return "", err
		}
		children = c
	}

	var match *image.Image
	for _, child := range children {
		childImage, err := ic.imageService.GetImage(ctx, child.String(), imagetype.GetImageOpts{})
		if err != nil {
			return "", err
		}

		if isMatch(&childImage.ContainerConfig, cfg) {
			if childImage.Created != nil && (match == nil || match.Created.Before(*childImage.Created)) {
				match = childImage
			}
		}
	}

	if match == nil {
		return "", nil
	}

	return match.ID().String(), nil
}

type imageCache struct {
	images       []*image.Image
	imageService *ImageService
	lc           *localCache
}

func (ic *imageCache) GetCache(parentID string, cfg *container.Config) (imageID string, err error) {
	ctx := context.TODO()

	imgID, err := ic.lc.GetCache(parentID, cfg)
	if err != nil {
		return "", err
	}
	if imgID != "" {
		for _, s := range ic.images {
			if ic.isParent(ctx, s, image.ID(imgID)) {
				return imgID, nil
			}
		}
	}

	var parent *image.Image
	lenHistory := 0

	if parentID != "" {
		parent, err = ic.imageService.GetImage(ctx, parentID, imagetype.GetImageOpts{})
		if err != nil {
			return "", err
		}
		lenHistory = len(parent.History)
	}
	for _, target := range ic.images {
		if !isValidParent(target, parent) || !isValidConfig(cfg, target.History[lenHistory]) {
			continue
		}
		return target.ID().String(), nil
	}

	return "", nil
}

func isValidConfig(cfg *container.Config, h image.History) bool {
	// todo: make this format better than join that loses data
	return strings.Join(cfg.Cmd, " ") == h.CreatedBy
}

func isValidParent(img, parent *image.Image) bool {
	if len(img.History) == 0 {
		return false
	}
	if parent == nil || len(parent.History) == 0 && len(parent.RootFS.DiffIDs) == 0 {
		return true
	}
	if len(parent.History) >= len(img.History) {
		return false
	}
	if len(parent.RootFS.DiffIDs) > len(img.RootFS.DiffIDs) {
		return false
	}

	for i, h := range parent.History {
		if !reflect.DeepEqual(h, img.History[i]) {
			return false
		}
	}
	for i, d := range parent.RootFS.DiffIDs {
		if d != img.RootFS.DiffIDs[i] {
			return false
		}
	}
	return true
}

func (ic *imageCache) isParent(ctx context.Context, img *image.Image, parentID image.ID) bool {
	ii, err := ic.imageService.resolveImage(ctx, img.ImageID())
	if err != nil {
		return false
	}
	parent, ok := ii.Labels[imageLabelClassicBuilderParent]
	if ok {
		return parent == parentID.String()
	}

	p, err := ic.imageService.GetImage(ctx, parentID.String(), imagetype.GetImageOpts{})
	if err != nil {
		return false
	}
	return ic.isParent(ctx, p, parentID)
}

// compare two Config struct. Do not compare the "Image" nor "Hostname" fields
// If OpenStdin is set, then it differs
func isMatch(a, b *container.Config) bool {
	if a == nil || b == nil ||
		a.OpenStdin || b.OpenStdin {
		return false
	}
	if a.AttachStdout != b.AttachStdout ||
		a.AttachStderr != b.AttachStderr ||
		a.User != b.User ||
		a.OpenStdin != b.OpenStdin ||
		a.Tty != b.Tty {
		return false
	}

	if len(a.Cmd) != len(b.Cmd) ||
		len(a.Env) != len(b.Env) ||
		len(a.Labels) != len(b.Labels) ||
		len(a.ExposedPorts) != len(b.ExposedPorts) ||
		len(a.Entrypoint) != len(b.Entrypoint) ||
		len(a.Volumes) != len(b.Volumes) {
		return false
	}

	for i := 0; i < len(a.Cmd); i++ {
		if a.Cmd[i] != b.Cmd[i] {
			return false
		}
	}
	for i := 0; i < len(a.Env); i++ {
		if a.Env[i] != b.Env[i] {
			return false
		}
	}
	for k, v := range a.Labels {
		if v != b.Labels[k] {
			return false
		}
	}
	for k := range a.ExposedPorts {
		if _, exists := b.ExposedPorts[k]; !exists {
			return false
		}
	}

	for i := 0; i < len(a.Entrypoint); i++ {
		if a.Entrypoint[i] != b.Entrypoint[i] {
			return false
		}
	}
	for key := range a.Volumes {
		if _, exists := b.Volumes[key]; !exists {
			return false
		}
	}
	return true
}
