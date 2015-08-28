package daemon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/parsers/filters"
)

// List returns an array of all containers registered in the daemon.
func (daemon *Daemon) List() []*Container {
	return daemon.containers.List()
}

// ContainersConfig is a struct for configuring the command to list
// containers.
type ContainersConfig struct {
	// if true show all containers, otherwise only running containers.
	All bool
	// show all containers created after this container id
	Since string
	// show all containers created before this container id
	Before string
	// number of containers to return at most
	Limit int
	// if true include the sizes of the containers
	Size bool
	// return only containers that match filters
	Filters string
}

// Containers returns a list of all the containers.
func (daemon *Daemon) Containers(config *ContainersConfig) ([]*types.Container, error) {
	var (
		foundBefore    bool
		displayed      int
		ancestorFilter bool
		all            = config.All
		n              = config.Limit
		psFilters      filters.Args
		filtExited     []int
	)
	imagesFilter := map[string]bool{}
	containers := []*types.Container{}

	psFilters, err := filters.FromParam(config.Filters)
	if err != nil {
		return nil, err
	}
	if i, ok := psFilters["exited"]; ok {
		for _, value := range i {
			code, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			filtExited = append(filtExited, code)
		}
	}

	if i, ok := psFilters["status"]; ok {
		for _, value := range i {
			if !isValidStateString(value) {
				return nil, errors.New("Unrecognised filter value for status")
			}
			if value == "exited" || value == "created" {
				all = true
			}
		}
	}

	if ancestors, ok := psFilters["ancestor"]; ok {
		ancestorFilter = true
		byParents := daemon.Graph().ByParent()
		// The idea is to walk the graph down the most "efficient" way.
		for _, ancestor := range ancestors {
			// First, get the imageId of the ancestor filter (yay)
			image, err := daemon.Repositories().LookupImage(ancestor)
			if err != nil {
				logrus.Warnf("Error while looking up for image %v", ancestor)
				continue
			}
			if imagesFilter[ancestor] {
				// Already seen this ancestor, skip it
				continue
			}
			// Then walk down the graph and put the imageIds in imagesFilter
			populateImageFilterByParents(imagesFilter, image.ID, byParents)
		}
	}

	names := map[string][]string{}
	daemon.containerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, 1)

	var beforeCont, sinceCont *Container
	if config.Before != "" {
		beforeCont, err = daemon.Get(config.Before)
		if err != nil {
			return nil, err
		}
	}

	if config.Since != "" {
		sinceCont, err = daemon.Get(config.Since)
		if err != nil {
			return nil, err
		}
	}

	errLast := errors.New("last container")
	writeCont := func(container *Container) error {
		container.Lock()
		defer container.Unlock()
		if !container.Running && !all && n <= 0 && config.Since == "" && config.Before == "" {
			return nil
		}
		if !psFilters.Match("name", container.Name) {
			return nil
		}

		if !psFilters.Match("id", container.ID) {
			return nil
		}

		if !psFilters.MatchKVList("label", container.Config.Labels) {
			return nil
		}

		if config.Before != "" && !foundBefore {
			if container.ID == beforeCont.ID {
				foundBefore = true
			}
			return nil
		}
		if n > 0 && displayed == n {
			return errLast
		}
		if config.Since != "" {
			if container.ID == sinceCont.ID {
				return errLast
			}
		}
		if len(filtExited) > 0 {
			shouldSkip := true
			for _, code := range filtExited {
				if code == container.ExitCode && !container.Running {
					shouldSkip = false
					break
				}
			}
			if shouldSkip {
				return nil
			}
		}

		if !psFilters.Match("status", container.State.StateString()) {
			return nil
		}

		if ancestorFilter {
			if len(imagesFilter) == 0 {
				return nil
			}
			if !imagesFilter[container.ImageID] {
				return nil
			}
		}

		displayed++
		newC := &types.Container{
			ID:    container.ID,
			Names: names[container.ID],
		}

		img, err := daemon.Repositories().LookupImage(container.Config.Image)
		if err != nil {
			// If the image can no longer be found by its original reference,
			// it makes sense to show the ID instead of a stale reference.
			newC.Image = container.ImageID
		} else if container.ImageID == img.ID {
			newC.Image = container.Config.Image
		} else {
			newC.Image = container.ImageID
		}

		if len(container.Args) > 0 {
			args := []string{}
			for _, arg := range container.Args {
				if strings.Contains(arg, " ") {
					args = append(args, fmt.Sprintf("'%s'", arg))
				} else {
					args = append(args, arg)
				}
			}
			argsAsString := strings.Join(args, " ")

			newC.Command = fmt.Sprintf("%s %s", container.Path, argsAsString)
		} else {
			newC.Command = fmt.Sprintf("%s", container.Path)
		}
		newC.Created = container.Created.Unix()
		newC.Status = container.State.String()
		newC.HostConfig.NetworkMode = string(container.hostConfig.NetworkMode)

		newC.Ports = []types.Port{}
		for port, bindings := range container.NetworkSettings.Ports {
			p, err := nat.ParsePort(port.Port())
			if err != nil {
				return err
			}
			if len(bindings) == 0 {
				newC.Ports = append(newC.Ports, types.Port{
					PrivatePort: p,
					Type:        port.Proto(),
				})
				continue
			}
			for _, binding := range bindings {
				h, err := nat.ParsePort(binding.HostPort)
				if err != nil {
					return err
				}
				newC.Ports = append(newC.Ports, types.Port{
					PrivatePort: p,
					PublicPort:  h,
					Type:        port.Proto(),
					IP:          binding.HostIP,
				})
			}
		}

		if config.Size {
			sizeRw, sizeRootFs := container.getSize()
			newC.SizeRw = sizeRw
			newC.SizeRootFs = sizeRootFs
		}
		newC.Labels = container.Config.Labels
		containers = append(containers, newC)
		return nil
	}

	for _, container := range daemon.List() {
		if err := writeCont(container); err != nil {
			if err != errLast {
				return nil, err
			}
			break
		}
	}
	return containers, nil
}

// Volumes lists known volumes, using the filter to restrict the range
// of volumes returned.
func (daemon *Daemon) Volumes(filter string) ([]*types.Volume, error) {
	var volumesOut []*types.Volume
	volFilters, err := filters.FromParam(filter)
	if err != nil {
		return nil, err
	}

	filterUsed := false
	if i, ok := volFilters["dangling"]; ok {
		if len(i) > 1 {
			return nil, fmt.Errorf("Conflict: cannot use more than 1 value for `dangling` filter")
		}

		filterValue := i[0]
		if strings.ToLower(filterValue) == "true" || filterValue == "1" {
			filterUsed = true
		}
	}

	volumes := daemon.volumes.List()
	for _, v := range volumes {
		if filterUsed && daemon.volumes.Count(v) == 0 {
			continue
		}
		volumesOut = append(volumesOut, volumeToAPIType(v))
	}
	return volumesOut, nil
}

func populateImageFilterByParents(ancestorMap map[string]bool, imageID string, byParents map[string][]*image.Image) {
	if !ancestorMap[imageID] {
		if images, ok := byParents[imageID]; ok {
			for _, image := range images {
				populateImageFilterByParents(ancestorMap, image.ID, byParents)
			}
		}
		ancestorMap[imageID] = true
	}
}
