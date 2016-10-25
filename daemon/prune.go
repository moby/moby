package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/volume"
)

// ContainersPrune removes unused containers
func (daemon *Daemon) ContainersPrune(config *types.ContainersPruneConfig) (*types.ContainersPruneReport, error) {
	rep := &types.ContainersPruneReport{}

	allContainers := daemon.List()
	for _, c := range allContainers {
		if !c.IsRunning() {
			cSize, _ := daemon.getSize(c)
			// TODO: sets RmLink to true?
			err := daemon.ContainerRm(c.ID, &types.ContainerRmConfig{})
			if err != nil {
				logrus.Warnf("failed to prune container %s: %v", c.ID, err)
				continue
			}
			if cSize > 0 {
				rep.SpaceReclaimed += uint64(cSize)
			}
			rep.ContainersDeleted = append(rep.ContainersDeleted, c.ID)
		}
	}

	return rep, nil
}

// VolumesPrune removes unused local volumes
func (daemon *Daemon) VolumesPrune(config *types.VolumesPruneConfig) (*types.VolumesPruneReport, error) {
	rep := &types.VolumesPruneReport{}

	pruneVols := func(v volume.Volume) error {
		name := v.Name()
		refs := daemon.volumes.Refs(v)

		if len(refs) == 0 {
			vSize, err := directory.Size(v.Path())
			if err != nil {
				logrus.Warnf("could not determine size of volume %s: %v", name, err)
			}
			err = daemon.volumes.Remove(v)
			if err != nil {
				logrus.Warnf("could not remove volume %s: %v", name, err)
				return nil
			}
			rep.SpaceReclaimed += uint64(vSize)
			rep.VolumesDeleted = append(rep.VolumesDeleted, name)
		}

		return nil
	}

	err := daemon.traverseLocalVolumes(pruneVols)

	return rep, err
}

// ImagesPrune removes unused images
func (daemon *Daemon) ImagesPrune(config *types.ImagesPruneConfig) (*types.ImagesPruneReport, error) {
	rep := &types.ImagesPruneReport{}

	var allImages map[image.ID]*image.Image
	if config.DanglingOnly {
		allImages = daemon.imageStore.Heads()
	} else {
		allImages = daemon.imageStore.Map()
	}
	allContainers := daemon.List()
	imageRefs := map[string]bool{}
	for _, c := range allContainers {
		imageRefs[c.ID] = true
	}

	// Filter intermediary images and get their unique size
	allLayers := daemon.layerStore.Map()
	topImages := map[image.ID]*image.Image{}
	for id, img := range allImages {
		dgst := digest.Digest(id)
		if len(daemon.referenceStore.References(dgst)) == 0 && len(daemon.imageStore.Children(id)) != 0 {
			continue
		}
		topImages[id] = img
	}

	for id := range topImages {
		dgst := digest.Digest(id)
		hex := dgst.Hex()
		if _, ok := imageRefs[hex]; ok {
			continue
		}

		deletedImages := []types.ImageDelete{}
		refs := daemon.referenceStore.References(dgst)
		if len(refs) > 0 {
			if config.DanglingOnly {
				// Not a dangling image
				continue
			}

			nrRefs := len(refs)
			for _, ref := range refs {
				// If nrRefs == 1, we have an image marked as myreponame:<none>
				// i.e. the tag content was changed
				if _, ok := ref.(reference.Canonical); ok && nrRefs > 1 {
					continue
				}
				imgDel, err := daemon.ImageDelete(ref.String(), false, true)
				if err != nil {
					logrus.Warnf("could not delete reference %s: %v", ref.String(), err)
					continue
				}
				deletedImages = append(deletedImages, imgDel...)
			}
		} else {
			imgDel, err := daemon.ImageDelete(hex, false, true)
			if err != nil {
				logrus.Warnf("could not delete image %s: %v", hex, err)
				continue
			}
			deletedImages = append(deletedImages, imgDel...)
		}

		rep.ImagesDeleted = append(rep.ImagesDeleted, deletedImages...)
	}

	// Compute how much space was freed
	for _, d := range rep.ImagesDeleted {
		if d.Deleted != "" {
			chid := layer.ChainID(d.Deleted)
			if l, ok := allLayers[chid]; ok {
				diffSize, err := l.DiffSize()
				if err != nil {
					logrus.Warnf("failed to get layer %s size: %v", chid, err)
					continue
				}
				rep.SpaceReclaimed += uint64(diffSize)
			}
		}
	}

	return rep, nil
}
