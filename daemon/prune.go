package daemon

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/libnetwork"
	digest "github.com/opencontainers/go-digest"
)

// ContainersPrune removes unused containers
func (daemon *Daemon) ContainersPrune(pruneFilters filters.Args) (*types.ContainersPruneReport, error) {
	rep := &types.ContainersPruneReport{}

	until, err := getUntilFromPruneFilters(pruneFilters)
	if err != nil {
		return nil, err
	}

	allContainers := daemon.List()
	for _, c := range allContainers {
		if !c.IsRunning() {
			if !until.IsZero() && c.Created.After(until) {
				continue
			}
			cSize, _ := daemon.getSize(c.ID)
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
func (daemon *Daemon) VolumesPrune(pruneFilters filters.Args) (*types.VolumesPruneReport, error) {
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
func (daemon *Daemon) ImagesPrune(pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	rep := &types.ImagesPruneReport{}

	danglingOnly := true
	if pruneFilters.Include("dangling") {
		if pruneFilters.ExactMatch("dangling", "false") || pruneFilters.ExactMatch("dangling", "0") {
			danglingOnly = false
		} else if !pruneFilters.ExactMatch("dangling", "true") && !pruneFilters.ExactMatch("dangling", "1") {
			return nil, fmt.Errorf("Invalid filter 'dangling=%s'", pruneFilters.Get("dangling"))
		}
	}

	until, err := getUntilFromPruneFilters(pruneFilters)
	if err != nil {
		return nil, err
	}

	var allImages map[image.ID]*image.Image
	if danglingOnly {
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
		if !until.IsZero() && img.Created.After(until) {
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

		deletedImages := []types.ImageDeleteResponseItem{}
		refs := daemon.referenceStore.References(dgst)
		if len(refs) > 0 {
			shouldDelete := !danglingOnly
			if !shouldDelete {
				hasTag := false
				for _, ref := range refs {
					if _, ok := ref.(reference.NamedTagged); ok {
						hasTag = true
						break
					}
				}

				// Only delete if it's untagged (i.e. repo:<none>)
				shouldDelete = !hasTag
			}

			if shouldDelete {
				for _, ref := range refs {
					imgDel, err := daemon.ImageDelete(ref.String(), false, true)
					if err != nil {
						logrus.Warnf("could not delete reference %s: %v", ref.String(), err)
						continue
					}
					deletedImages = append(deletedImages, imgDel...)
				}
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

// localNetworksPrune removes unused local networks
func (daemon *Daemon) localNetworksPrune(pruneFilters filters.Args) *types.NetworksPruneReport {
	rep := &types.NetworksPruneReport{}

	until, _ := getUntilFromPruneFilters(pruneFilters)

	// When the function returns true, the walk will stop.
	l := func(nw libnetwork.Network) bool {
		if !until.IsZero() && nw.Info().Created().After(until) {
			return false
		}
		nwName := nw.Name()
		if runconfig.IsPreDefinedNetwork(nwName) {
			return false
		}
		if len(nw.Endpoints()) > 0 {
			return false
		}
		if err := daemon.DeleteNetwork(nw.ID()); err != nil {
			logrus.Warnf("could not remove local network %s: %v", nwName, err)
			return false
		}
		rep.NetworksDeleted = append(rep.NetworksDeleted, nwName)
		return false
	}
	daemon.netController.WalkNetworks(l)
	return rep
}

// clusterNetworksPrune removes unused cluster networks
func (daemon *Daemon) clusterNetworksPrune(pruneFilters filters.Args) (*types.NetworksPruneReport, error) {
	rep := &types.NetworksPruneReport{}

	until, _ := getUntilFromPruneFilters(pruneFilters)

	cluster := daemon.GetCluster()

	if !cluster.IsManager() {
		return rep, nil
	}

	networks, err := cluster.GetNetworks()
	if err != nil {
		return rep, err
	}
	networkIsInUse := regexp.MustCompile(`network ([[:alnum:]]+) is in use`)
	for _, nw := range networks {
		if nw.Ingress {
			// Routing-mesh network removal has to be explicitly invoked by user
			continue
		}
		if !until.IsZero() && nw.Created.After(until) {
			continue
		}
		// https://github.com/docker/docker/issues/24186
		// `docker network inspect` unfortunately displays ONLY those containers that are local to that node.
		// So we try to remove it anyway and check the error
		err = cluster.RemoveNetwork(nw.ID)
		if err != nil {
			// we can safely ignore the "network .. is in use" error
			match := networkIsInUse.FindStringSubmatch(err.Error())
			if len(match) != 2 || match[1] != nw.ID {
				logrus.Warnf("could not remove cluster network %s: %v", nw.Name, err)
			}
			continue
		}
		rep.NetworksDeleted = append(rep.NetworksDeleted, nw.Name)
	}
	return rep, nil
}

// NetworksPrune removes unused networks
func (daemon *Daemon) NetworksPrune(pruneFilters filters.Args) (*types.NetworksPruneReport, error) {
	if _, err := getUntilFromPruneFilters(pruneFilters); err != nil {
		return nil, err
	}

	clusterRep, err := daemon.clusterNetworksPrune(pruneFilters)
	if err != nil {
		return nil, fmt.Errorf("could not remove cluster networks: %s", err)
	}
	rep := &types.NetworksPruneReport{}
	rep.NetworksDeleted = append(rep.NetworksDeleted, clusterRep.NetworksDeleted...)

	localRep := daemon.localNetworksPrune(pruneFilters)
	rep.NetworksDeleted = append(rep.NetworksDeleted, localRep.NetworksDeleted...)
	return rep, nil
}

func getUntilFromPruneFilters(pruneFilters filters.Args) (time.Time, error) {
	until := time.Time{}
	if !pruneFilters.Include("until") {
		return until, nil
	}
	untilFilters := pruneFilters.Get("until")
	if len(untilFilters) > 1 {
		return until, fmt.Errorf("more than one until filter specified")
	}
	ts, err := timetypes.GetTimestamp(untilFilters[0], time.Now())
	if err != nil {
		return until, err
	}
	seconds, nanoseconds, err := timetypes.ParseTimestamps(ts, 0)
	if err != nil {
		return until, err
	}
	until = time.Unix(seconds, nanoseconds)
	return until, nil
}
