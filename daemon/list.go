package daemon

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/volume"
	"github.com/docker/go-connections/nat"
)

var acceptedVolumeFilterTags = map[string]bool{
	"dangling": true,
	"name":     true,
	"driver":   true,
	"label":    true,
}

var acceptedPsFilterTags = map[string]bool{
	"ancestor":  true,
	"before":    true,
	"exited":    true,
	"id":        true,
	"isolation": true,
	"label":     true,
	"name":      true,
	"status":    true,
	"health":    true,
	"since":     true,
	"volume":    true,
	"network":   true,
	"is-task":   true,
	"publish":   true,
	"expose":    true,
}

// iterationAction represents possible outcomes happening during the container iteration.
type iterationAction int

// containerReducer represents a reducer for a container.
// Returns the object to serialize by the api.
type containerReducer func(*container.Container, *listContext) (*types.Container, error)

const (
	// includeContainer is the action to include a container in the reducer.
	includeContainer iterationAction = iota
	// excludeContainer is the action to exclude a container in the reducer.
	excludeContainer
	// stopIteration is the action to stop iterating over the list of containers.
	stopIteration
)

// errStopIteration makes the iterator to stop without returning an error.
var errStopIteration = errors.New("container list iteration stopped")

// List returns an array of all containers registered in the daemon.
func (daemon *Daemon) List() []*container.Container {
	return daemon.containers.List()
}

// listContext is the daemon generated filtering to iterate over containers.
// This is created based on the user specification from types.ContainerListOptions.
type listContext struct {
	// idx is the container iteration index for this context
	idx int
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
	beforeFilter *container.Container
	// sinceFilter is a filter to stop the filtering when the iterator arrive to the given container
	sinceFilter *container.Container

	// taskFilter tells if we should filter based on wether a container is part of a task
	taskFilter bool
	// isTask tells us if the we should filter container that are a task (true) or not (false)
	isTask bool

	// publish is a list of published ports to filter with
	publish map[nat.Port]bool
	// expose is a list of exposed ports to filter with
	expose map[nat.Port]bool

	// ContainerListOptions is the filters set by the user
	*types.ContainerListOptions
}

// byContainerCreated is a temporary type used to sort a list of containers by creation time.
type byContainerCreated []*container.Container

func (r byContainerCreated) Len() int      { return len(r) }
func (r byContainerCreated) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byContainerCreated) Less(i, j int) bool {
	return r[i].Created.UnixNano() < r[j].Created.UnixNano()
}

// Containers returns the list of containers to show given the user's filtering.
func (daemon *Daemon) Containers(config *types.ContainerListOptions) ([]*types.Container, error) {
	return daemon.reduceContainers(config, daemon.transformContainer)
}

func (daemon *Daemon) filterByNameIDMatches(ctx *listContext) []*container.Container {
	idSearch := false
	names := ctx.filters.Get("name")
	ids := ctx.filters.Get("id")
	if len(names)+len(ids) == 0 {
		// if name or ID filters are not in use, return to
		// standard behavior of walking the entire container
		// list from the daemon's in-memory store
		return daemon.List()
	}

	// idSearch will determine if we limit name matching to the IDs
	// matched from any IDs which were specified as filters
	if len(ids) > 0 {
		idSearch = true
	}

	matches := make(map[string]bool)
	// find ID matches; errors represent "not found" and can be ignored
	for _, id := range ids {
		if fullID, err := daemon.idIndex.Get(id); err == nil {
			matches[fullID] = true
		}
	}

	// look for name matches; if ID filtering was used, then limit the
	// search space to the matches map only; errors represent "not found"
	// and can be ignored
	if len(names) > 0 {
		for id, idNames := range ctx.names {
			// if ID filters were used and no matches on that ID were
			// found, continue to next ID in the list
			if idSearch && !matches[id] {
				continue
			}
			for _, eachName := range idNames {
				if ctx.filters.Match("name", eachName) {
					matches[id] = true
				}
			}
		}
	}

	cntrs := make([]*container.Container, 0, len(matches))
	for id := range matches {
		if c := daemon.containers.Get(id); c != nil {
			cntrs = append(cntrs, c)
		}
	}

	// Restore sort-order after filtering
	// Created gives us nanosec resolution for sorting
	sort.Sort(sort.Reverse(byContainerCreated(cntrs)))

	return cntrs
}

// reduceContainers parses the user's filtering options and generates the list of containers to return based on a reducer.
func (daemon *Daemon) reduceContainers(config *types.ContainerListOptions, reducer containerReducer) ([]*types.Container, error) {
	var (
		containers = []*types.Container{}
	)

	ctx, err := daemon.foldFilter(config)
	if err != nil {
		return nil, err
	}

	// fastpath to only look at a subset of containers if specific name
	// or ID matches were provided by the user--otherwise we potentially
	// end up locking and querying many more containers than intended
	containerList := daemon.filterByNameIDMatches(ctx)

	for _, container := range containerList {
		t, err := daemon.reducePsContainer(container, ctx, reducer)
		if err != nil {
			if err != errStopIteration {
				return nil, err
			}
			break
		}
		if t != nil {
			containers = append(containers, t)
			ctx.idx++
		}
	}

	return containers, nil
}

// reducePsContainer is the basic representation for a container as expected by the ps command.
func (daemon *Daemon) reducePsContainer(container *container.Container, ctx *listContext, reducer containerReducer) (*types.Container, error) {
	container.Lock()
	defer container.Unlock()

	// filter containers to return
	action := includeContainerInList(container, ctx)
	switch action {
	case excludeContainer:
		return nil, nil
	case stopIteration:
		return nil, errStopIteration
	}

	// transform internal container struct into api structs
	return reducer(container, ctx)
}

// foldFilter generates the container filter based on the user's filtering options.
func (daemon *Daemon) foldFilter(config *types.ContainerListOptions) (*listContext, error) {
	psFilters := config.Filters

	if err := psFilters.Validate(acceptedPsFilterTags); err != nil {
		return nil, err
	}

	var filtExited []int

	err := psFilters.WalkValues("exited", func(value string) error {
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

	var taskFilter, isTask bool
	if psFilters.Include("is-task") {
		if psFilters.ExactMatch("is-task", "true") {
			taskFilter = true
			isTask = true
		} else if psFilters.ExactMatch("is-task", "false") {
			taskFilter = true
			isTask = false
		} else {
			return nil, fmt.Errorf("Invalid filter 'is-task=%s'", psFilters.Get("is-task"))
		}
	}

	err = psFilters.WalkValues("health", func(value string) error {
		if !container.IsValidHealthString(value) {
			return fmt.Errorf("Unrecognised filter value for health: %s", value)
		}

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

	publishFilter := map[nat.Port]bool{}
	err = psFilters.WalkValues("publish", func(value string) error {
		if strings.Contains(value, ":") {
			return fmt.Errorf("filter for 'publish' should not contain ':': %v", value)
		}
		//support two formats, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(value)
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return fmt.Errorf("error while looking up for publish %v: %s", value, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return fmt.Errorf("error while looking up for publish %v: %s", value, err)
			}
			publishFilter[p] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	exposeFilter := map[nat.Port]bool{}
	err = psFilters.WalkValues("expose", func(value string) error {
		if strings.Contains(value, ":") {
			return fmt.Errorf("filter for 'expose' should not contain ':': %v", value)
		}
		//support two formats, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(value)
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return fmt.Errorf("error while looking up for 'expose' %v: %s", value, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return fmt.Errorf("error while looking up for 'expose' %v: %s", value, err)
			}
			exposeFilter[p] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &listContext{
		filters:              psFilters,
		ancestorFilter:       ancestorFilter,
		images:               imagesFilter,
		exitAllowed:          filtExited,
		beforeFilter:         beforeContFilter,
		sinceFilter:          sinceContFilter,
		taskFilter:           taskFilter,
		isTask:               isTask,
		publish:              publishFilter,
		expose:               exposeFilter,
		ContainerListOptions: config,
		names:                daemon.nameIndex.GetAll(),
	}, nil
}

// includeContainerInList decides whether a container should be included in the output or not based in the filter.
// It also decides if the iteration should be stopped or not.
func includeContainerInList(container *container.Container, ctx *listContext) iterationAction {
	// Do not include container if it's in the list before the filter container.
	// Set the filter container to nil to include the rest of containers after this one.
	if ctx.beforeFilter != nil {
		if container.ID == ctx.beforeFilter.ID {
			ctx.beforeFilter = nil
		}
		return excludeContainer
	}

	// Stop iteration when the container arrives to the filter container
	if ctx.sinceFilter != nil {
		if container.ID == ctx.sinceFilter.ID {
			return stopIteration
		}
	}

	// Do not include container if it's stopped and we're not filters
	if !container.Running && !ctx.All && ctx.Limit <= 0 {
		return excludeContainer
	}

	// Do not include container if the name doesn't match
	if !ctx.filters.Match("name", container.Name) {
		return excludeContainer
	}

	// Do not include container if the id doesn't match
	if !ctx.filters.Match("id", container.ID) {
		return excludeContainer
	}

	if ctx.taskFilter {
		if ctx.isTask != container.Managed {
			return excludeContainer
		}
	}

	// Do not include container if any of the labels don't match
	if !ctx.filters.MatchKVList("label", container.Config.Labels) {
		return excludeContainer
	}

	// Do not include container if isolation doesn't match
	if excludeContainer == excludeByIsolation(container, ctx) {
		return excludeContainer
	}

	// Stop iteration when the index is over the limit
	if ctx.Limit > 0 && ctx.idx == ctx.Limit {
		return stopIteration
	}

	// Do not include container if its exit code is not in the filter
	if len(ctx.exitAllowed) > 0 {
		shouldSkip := true
		for _, code := range ctx.exitAllowed {
			if code == container.ExitCode() && !container.Running && !container.StartedAt.IsZero() {
				shouldSkip = false
				break
			}
		}
		if shouldSkip {
			return excludeContainer
		}
	}

	// Do not include container if its status doesn't match the filter
	if !ctx.filters.Match("status", container.State.StateString()) {
		return excludeContainer
	}

	// Do not include container if its health doesn't match the filter
	if !ctx.filters.ExactMatch("health", container.State.HealthString()) {
		return excludeContainer
	}

	if ctx.filters.Include("volume") {
		volumesByName := make(map[string]*volume.MountPoint)
		for _, m := range container.MountPoints {
			if m.Name != "" {
				volumesByName[m.Name] = m
			} else {
				volumesByName[m.Source] = m
			}
		}

		volumeExist := fmt.Errorf("volume mounted in container")
		err := ctx.filters.WalkValues("volume", func(value string) error {
			if _, exist := container.MountPoints[value]; exist {
				return volumeExist
			}
			if _, exist := volumesByName[value]; exist {
				return volumeExist
			}
			return nil
		})
		if err != volumeExist {
			return excludeContainer
		}
	}

	if ctx.ancestorFilter {
		if len(ctx.images) == 0 {
			return excludeContainer
		}
		if !ctx.images[container.ImageID] {
			return excludeContainer
		}
	}

	networkExist := fmt.Errorf("container part of network")
	if ctx.filters.Include("network") {
		err := ctx.filters.WalkValues("network", func(value string) error {
			if _, ok := container.NetworkSettings.Networks[value]; ok {
				return networkExist
			}
			for _, nw := range container.NetworkSettings.Networks {
				if nw.EndpointSettings == nil {
					continue
				}
				if nw.NetworkID == value {
					return networkExist
				}
			}
			return nil
		})
		if err != networkExist {
			return excludeContainer
		}
	}

	if len(ctx.publish) > 0 {
		shouldSkip := true
		for port := range ctx.publish {
			if _, ok := container.HostConfig.PortBindings[port]; ok {
				shouldSkip = false
				break
			}
		}
		if shouldSkip {
			return excludeContainer
		}
	}

	if len(ctx.expose) > 0 {
		shouldSkip := true
		for port := range ctx.expose {
			if _, ok := container.Config.ExposedPorts[port]; ok {
				shouldSkip = false
				break
			}
		}
		if shouldSkip {
			return excludeContainer
		}
	}

	return includeContainer
}

// transformContainer generates the container type expected by the docker ps command.
func (daemon *Daemon) transformContainer(container *container.Container, ctx *listContext) (*types.Container, error) {
	newC := &types.Container{
		ID:      container.ID,
		Names:   ctx.names[container.ID],
		ImageID: container.ImageID.String(),
	}
	if newC.Names == nil {
		// Dead containers will often have no name, so make sure the response isn't null
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
	newC.State = container.State.StateString()
	newC.Status = container.State.String()
	newC.HostConfig.NetworkMode = string(container.HostConfig.NetworkMode)
	// copy networks to avoid races
	networks := make(map[string]*networktypes.EndpointSettings)
	for name, network := range container.NetworkSettings.Networks {
		if network == nil || network.EndpointSettings == nil {
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
			NetworkID:           network.NetworkID,
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
				PrivatePort: uint16(p),
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
				PrivatePort: uint16(p),
				PublicPort:  uint16(h),
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
	newC.Mounts = addMountPoints(container)

	return newC, nil
}

// Volumes lists known volumes, using the filter to restrict the range
// of volumes returned.
func (daemon *Daemon) Volumes(filter string) ([]*types.Volume, []string, error) {
	var (
		volumesOut []*types.Volume
	)
	volFilters, err := filters.FromParam(filter)
	if err != nil {
		return nil, nil, err
	}

	if err := volFilters.Validate(acceptedVolumeFilterTags); err != nil {
		return nil, nil, err
	}

	volumes, warnings, err := daemon.volumes.List()
	if err != nil {
		return nil, nil, err
	}

	filterVolumes, err := daemon.filterVolumes(volumes, volFilters)
	if err != nil {
		return nil, nil, err
	}
	for _, v := range filterVolumes {
		apiV := volumeToAPIType(v)
		if vv, ok := v.(interface {
			CachedPath() string
		}); ok {
			apiV.Mountpoint = vv.CachedPath()
		} else {
			apiV.Mountpoint = v.Path()
		}
		volumesOut = append(volumesOut, apiV)
	}
	return volumesOut, warnings, nil
}

// filterVolumes filters volume list according to user specified filter
// and returns user chosen volumes
func (daemon *Daemon) filterVolumes(vols []volume.Volume, filter filters.Args) ([]volume.Volume, error) {
	// if filter is empty, return original volume list
	if filter.Len() == 0 {
		return vols, nil
	}

	var retVols []volume.Volume
	for _, vol := range vols {
		if filter.Include("name") {
			if !filter.Match("name", vol.Name()) {
				continue
			}
		}
		if filter.Include("driver") {
			if !filter.ExactMatch("driver", vol.DriverName()) {
				continue
			}
		}
		if filter.Include("label") {
			v, ok := vol.(volume.DetailedVolume)
			if !ok {
				continue
			}
			if !filter.MatchKVList("label", v.Labels()) {
				continue
			}
		}
		retVols = append(retVols, vol)
	}
	danglingOnly := false
	if filter.Include("dangling") {
		if filter.ExactMatch("dangling", "true") || filter.ExactMatch("dangling", "1") {
			danglingOnly = true
		} else if !filter.ExactMatch("dangling", "false") && !filter.ExactMatch("dangling", "0") {
			return nil, fmt.Errorf("Invalid filter 'dangling=%s'", filter.Get("dangling"))
		}
		retVols = daemon.volumes.FilterByUsed(retVols, !danglingOnly)
	}
	return retVols, nil
}

func populateImageFilterByParents(ancestorMap map[image.ID]bool, imageID image.ID, getChildren func(image.ID) []image.ID) {
	if !ancestorMap[imageID] {
		for _, id := range getChildren(imageID) {
			populateImageFilterByParents(ancestorMap, id, getChildren)
		}
		ancestorMap[imageID] = true
	}
}
