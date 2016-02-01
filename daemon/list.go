package daemon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/go-connections/nat"
)

var acceptedVolumeFilterTags = map[string]bool{
	"dangling": true,
}

// ContainersConfig is the filtering specified by the user to iterate over containers.
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

// listContext is the daemon generated filtering to iterate over containers.
// This is created based on the user specification.
type listContext struct {
	// ancestorFilter tells whether it should check ancestors or not
	ancestorFilter bool
	// names is a list of container names to filter with
	names map[string][]string
	// images is a list of images to filter with
	images map[image.ID]bool
	// filters is a collection of arguments to filter with, specified by the user
	filters filters.Args
	// exitAllowed is a list of exit codes allowed to filter with
	exitAllowed []int
	// beforeFilter is a filter to ignore containers that appear before the one given
	// this is used for --filter=before= and --before=, the latter is deprecated.
	beforeFilter *container.Container
	// sinceFilter is a filter to stop the filtering when the iterator arrive to the given container
	// this is used for --filter=since= and --since=, the latter is deprecated.
	sinceFilter *container.Container
	// ContainersConfig is the filters set by the user
	*ContainersConfig
}

// Containers returns the list of containers to show given the user's filtering.
func (daemon *Daemon) Containers(config *ContainersConfig) ([]*types.Container, error) {
	// initialize containers as an empty slice
	// to return `[]` when there are no containers
	// rather than `null`, like it'd return
	// using `var containers []*types.Container`.
	containers := []*types.Container{}

	ctx, err := daemon.foldFilter(config)
	if err != nil {
		return nil, err
	}

	err = daemon.containers.ReduceAll(ctx.filterContainer, func(cont *container.Container) error {
		t, err := daemon.transformContainer(cont, ctx)
		if err != nil {
			return err
		}
		containers = append(containers, t)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(containers) > 0 {
		history := sortContainersByCreated(containers)
		sort.Sort(history)
		containers = history
		if ctx.Limit > 0 {
			containers = containers[:ctx.Limit]
		}
	}

	return containers, nil
}

// foldFilter generates the container filter based in the user's filtering options.
func (daemon *Daemon) foldFilter(config *ContainersConfig) (*listContext, error) {
	psFilters, err := filters.FromParam(config.Filters)
	if err != nil {
		return nil, err
	}

	var filtExited []int
	err = psFilters.WalkValues("exited", func(value string) error {
		code, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		filtExited = append(filtExited, code)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = psFilters.WalkValues("status", func(value string) error {
		if !container.IsValidStateString(value) {
			return fmt.Errorf("Unrecognised filter value for status: %s", value)
		}

		config.All = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	var beforeContFilter, sinceContFilter *container.Container
	err = psFilters.WalkValues("before", func(value string) error {
		beforeContFilter, err = daemon.GetContainer(value)
		return err
	})
	if err != nil {
		return nil, err
	}

	err = psFilters.WalkValues("since", func(value string) error {
		sinceContFilter, err = daemon.GetContainer(value)
		return err
	})
	if err != nil {
		return nil, err
	}

	imagesFilter := map[image.ID]bool{}
	var ancestorFilter bool
	if psFilters.Include("ancestor") {
		ancestorFilter = true
		psFilters.WalkValues("ancestor", func(ancestor string) error {
			id, err := daemon.GetImageID(ancestor)
			if err != nil {
				logrus.Warnf("Error while looking up for image %v", ancestor)
				return nil
			}
			if imagesFilter[id] {
				// Already seen this ancestor, skip it
				return nil
			}
			// Then walk down the graph and put the imageIds in imagesFilter
			populateImageFilterByParents(imagesFilter, id, daemon.imageStore.Children)
			return nil
		})
	}

	if config.Before != "" && beforeContFilter == nil {
		beforeContFilter, err = daemon.GetContainer(config.Before)
		if err != nil {
			return nil, err
		}
	}

	if config.Since != "" && sinceContFilter == nil {
		sinceContFilter, err = daemon.GetContainer(config.Since)
		if err != nil {
			return nil, err
		}
	}

	return &listContext{
		filters:          psFilters,
		ancestorFilter:   ancestorFilter,
		images:           imagesFilter,
		exitAllowed:      filtExited,
		beforeFilter:     beforeContFilter,
		sinceFilter:      sinceContFilter,
		ContainersConfig: config,
		names:            daemon.nameIndex.GetAll(),
	}, nil
}

// includeContainerInList decides whether a containers should be include in the output or not based in the filter.
// It also decides if the iteration should be stopped or not.
func (ctx *listContext) filterContainer(container *container.Container) bool {
	// Do not include container if it's stopped and we're not filters
	if !container.Running && !ctx.All && ctx.Limit <= 0 && ctx.beforeFilter == nil && ctx.sinceFilter == nil {
		return false
	}

	// Do not include container if the name doesn't match
	if !ctx.filters.Match("name", container.Name) {
		return false
	}

	// Do not include container if the id doesn't match
	if !ctx.filters.Match("id", container.ID) {
		return false
	}

	// Do not include container if any of the labels don't match
	if !ctx.filters.MatchKVList("label", container.Config.Labels) {
		return false
	}

	// Do not include container if the isolation mode doesn't match
	if excludeByIsolation(container.HostConfig.Isolation, ctx) {
		return false
	}

	// Do not include container if it's in the list before the filter container.
	// Set the filter container to nil to include the rest of containers after this one.
	if ctx.beforeFilter != nil {
		if container.ID == ctx.beforeFilter.ID {
			ctx.beforeFilter = nil
		}
		return false
	}

	// Stop iteration when the container arrives to the filter container
	if ctx.sinceFilter != nil {
		if container.ID == ctx.sinceFilter.ID {
			return false
		}
	}

	// Do not include container if its exit code is not in the filter
	if len(ctx.exitAllowed) > 0 {
		shouldSkip := true
		for _, code := range ctx.exitAllowed {
			if code == container.ExitCode && !container.Running {
				shouldSkip = false
				break
			}
		}
		if shouldSkip {
			return false
		}
	}

	// Do not include container if its status doesn't match the filter
	if !ctx.filters.Match("status", container.State.StateString()) {
		return false
	}

	if ctx.ancestorFilter {
		if len(ctx.images) == 0 {
			return false
		}
		if !ctx.images[container.ImageID] {
			return false
		}
	}

	return true
}

// transformContainer generates the container type expected by the docker ps command.
func (daemon *Daemon) transformContainer(container *container.Container, ctx *listContext) (*types.Container, error) {
	newC := &types.Container{
		ID:      container.ID,
		Names:   ctx.names[container.ID],
		ImageID: container.ImageID.String(),
	}
	if newC.Names == nil {
		// Dead containers will often have no name, so make sure the response isn't  null
		newC.Names = []string{}
	}

	image := container.Config.Image // if possible keep the original ref
	if image != container.ImageID.String() {
		id, err := daemon.GetImageID(image)
		if _, isDNE := err.(ErrImageDoesNotExist); err != nil && !isDNE {
			return nil, err
		}
		if err != nil || id != container.ImageID {
			image = container.ImageID.String()
		}
	}
	newC.Image = image

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
		newC.Command = container.Path
	}
	newC.Created = container.Created.Unix()
	newC.Status = container.State.String()
	newC.HostConfig.NetworkMode = string(container.HostConfig.NetworkMode)
	// copy networks to avoid races
	networks := make(map[string]*networktypes.EndpointSettings)
	for name, network := range container.NetworkSettings.Networks {
		if network == nil {
			continue
		}
		networks[name] = &networktypes.EndpointSettings{
			EndpointID:          network.EndpointID,
			Gateway:             network.Gateway,
			IPAddress:           network.IPAddress,
			IPPrefixLen:         network.IPPrefixLen,
			IPv6Gateway:         network.IPv6Gateway,
			GlobalIPv6Address:   network.GlobalIPv6Address,
			GlobalIPv6PrefixLen: network.GlobalIPv6PrefixLen,
			MacAddress:          network.MacAddress,
		}
		if network.IPAMConfig != nil {
			networks[name].IPAMConfig = &networktypes.EndpointIPAMConfig{
				IPv4Address: network.IPAMConfig.IPv4Address,
				IPv6Address: network.IPAMConfig.IPv6Address,
			}
		}
	}
	newC.NetworkSettings = &types.SummaryNetworkSettings{Networks: networks}

	newC.Ports = []types.Port{}
	for port, bindings := range container.NetworkSettings.Ports {
		p, err := nat.ParsePort(port.Port())
		if err != nil {
			return nil, err
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
				return nil, err
			}
			newC.Ports = append(newC.Ports, types.Port{
				PrivatePort: p,
				PublicPort:  h,
				Type:        port.Proto(),
				IP:          binding.HostIP,
			})
		}
	}

	if ctx.Size {
		sizeRw, sizeRootFs := daemon.getSize(container)
		newC.SizeRw = sizeRw
		newC.SizeRootFs = sizeRootFs
	}
	newC.Labels = container.Config.Labels

	return newC, nil
}

// Volumes lists known volumes, using the filter to restrict the range
// of volumes returned.
func (daemon *Daemon) Volumes(filter string) ([]*types.Volume, []string, error) {
	var (
		volumesOut   []*types.Volume
		danglingOnly = false
	)
	volFilters, err := filters.FromParam(filter)
	if err != nil {
		return nil, nil, err
	}

	if err := volFilters.Validate(acceptedVolumeFilterTags); err != nil {
		return nil, nil, err
	}

	if volFilters.Include("dangling") {
		if volFilters.ExactMatch("dangling", "true") || volFilters.ExactMatch("dangling", "1") {
			danglingOnly = true
		} else if !volFilters.ExactMatch("dangling", "false") && !volFilters.ExactMatch("dangling", "0") {
			return nil, nil, fmt.Errorf("Invalid filter 'dangling=%s'", volFilters.Get("dangling"))
		}
	}

	volumes, warnings, err := daemon.volumes.List()
	if err != nil {
		return nil, nil, err
	}
	if volFilters.Include("dangling") {
		volumes = daemon.volumes.FilterByUsed(volumes, !danglingOnly)
	}
	for _, v := range volumes {
		volumesOut = append(volumesOut, volumeToAPIType(v))
	}
	return volumesOut, warnings, nil
}

func populateImageFilterByParents(ancestorMap map[image.ID]bool, imageID image.ID, getChildren func(image.ID) []image.ID) {
	if !ancestorMap[imageID] {
		for _, id := range getChildren(imageID) {
			populateImageFilterByParents(ancestorMap, id, getChildren)
		}
		ancestorMap[imageID] = true
	}
}

type sortContainersByCreated []*types.Container

func (r sortContainersByCreated) Len() int           { return len(r) }
func (r sortContainersByCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r sortContainersByCreated) Less(i, j int) bool { return r[i].Created > r[j].Created }
