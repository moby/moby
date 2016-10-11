package daemon

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/volume"
)

func (daemon *Daemon) getLayerRefs() map[layer.ChainID]int {
	tmpImages := daemon.imageStore.Map()
	layerRefs := map[layer.ChainID]int{}
	for id, img := range tmpImages {
		dgst := digest.Digest(id)
		if len(daemon.referenceStore.References(dgst)) == 0 && len(daemon.imageStore.Children(id)) != 0 {
			continue
		}

		rootFS := *img.RootFS
		rootFS.DiffIDs = nil
		for _, id := range img.RootFS.DiffIDs {
			rootFS.Append(id)
			chid := rootFS.ChainID()
			layerRefs[chid]++
		}
	}

	return layerRefs
}

// SystemDiskUsage returns information about the daemon data disk usage
func (daemon *Daemon) SystemDiskUsage() (*types.DiskUsage, error) {
	// Retrieve container list
	allContainers, err := daemon.Containers(&types.ContainerListOptions{
		Size: true,
		All:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve container list: %v", err)
	}

	// Get all top images with extra attributes
	allImages, err := daemon.Images("", "", false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve image list: %v", err)
	}

	// Get all local volumes
	allVolumes := []*types.Volume{}
	getLocalVols := func(v volume.Volume) error {
		name := v.Name()
		refs := daemon.volumes.Refs(v)

		tv := volumeToAPIType(v)
		sz, err := directory.Size(v.Path())
		if err != nil {
			logrus.Warnf("failed to determine size of volume %v", name)
			sz = -1
		}
		tv.UsageData = &types.VolumeUsageData{Size: sz, RefCount: len(refs)}
		allVolumes = append(allVolumes, tv)

		return nil
	}

	err = daemon.traverseLocalVolumes(getLocalVols)
	if err != nil {
		return nil, err
	}

	// Get total layers size on disk
	layerRefs := daemon.getLayerRefs()
	allLayers := daemon.layerStore.Map()
	var allLayersSize int64
	for _, l := range allLayers {
		size, err := l.DiffSize()
		if err == nil {
			if _, ok := layerRefs[l.ChainID()]; ok {
				allLayersSize += size
			} else {
				logrus.Warnf("found leaked image layer %v", l.ChainID())
			}
		} else {
			logrus.Warnf("failed to get diff size for layer %v", l.ChainID())
		}

	}

	return &types.DiskUsage{
		LayersSize: allLayersSize,
		Containers: allContainers,
		Volumes:    allVolumes,
		Images:     allImages,
	}, nil
}
