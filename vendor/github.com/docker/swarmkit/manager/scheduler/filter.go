package scheduler

import (
	"fmt"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/constraint"
)

// Filter checks whether the given task can run on the given node.
// A filter may only operate
type Filter interface {
	// SetTask returns true when the filter is enabled for a given task
	// and assigns the task to the filter. It returns false if the filter
	// isn't applicable to this task.  For instance, a constraints filter
	// would return `false` if the task doesn't contain any constraints.
	SetTask(*api.Task) bool

	// Check returns true if the task assigned by SetTask can be scheduled
	// into the given node. This function should not be called if SetTask
	// returned false.
	Check(*NodeInfo) bool

	// Explain what a failure of this filter means
	Explain(nodes int) string
}

// ReadyFilter checks that the node is ready to schedule tasks.
type ReadyFilter struct {
}

// SetTask returns true when the filter is enabled for a given task.
func (f *ReadyFilter) SetTask(_ *api.Task) bool {
	return true
}

// Check returns true if the task can be scheduled into the given node.
func (f *ReadyFilter) Check(n *NodeInfo) bool {
	return n.Status.State == api.NodeStatus_READY &&
		n.Spec.Availability == api.NodeAvailabilityActive
}

// Explain returns an explanation of a failure.
func (f *ReadyFilter) Explain(nodes int) string {
	if nodes == 1 {
		return "1 node not available for new tasks"
	}
	return fmt.Sprintf("%d nodes not available for new tasks", nodes)
}

// ResourceFilter checks that the node has enough resources available to run
// the task.
type ResourceFilter struct {
	reservations *api.Resources
}

// SetTask returns true when the filter is enabled for a given task.
func (f *ResourceFilter) SetTask(t *api.Task) bool {
	r := t.Spec.Resources
	if r == nil || r.Reservations == nil {
		return false
	}
	if r.Reservations.NanoCPUs == 0 && r.Reservations.MemoryBytes == 0 {
		return false
	}
	f.reservations = r.Reservations
	return true
}

// Check returns true if the task can be scheduled into the given node.
func (f *ResourceFilter) Check(n *NodeInfo) bool {
	if f.reservations.NanoCPUs > n.AvailableResources.NanoCPUs {
		return false
	}

	if f.reservations.MemoryBytes > n.AvailableResources.MemoryBytes {
		return false
	}

	return true
}

// Explain returns an explanation of a failure.
func (f *ResourceFilter) Explain(nodes int) string {
	if nodes == 1 {
		return "insufficient resources on 1 node"
	}
	return fmt.Sprintf("insufficient resources on %d nodes", nodes)
}

// PluginFilter checks that the node has a specific volume plugin installed
type PluginFilter struct {
	t *api.Task
}

func referencesVolumePlugin(mount api.Mount) bool {
	return mount.Type == api.MountTypeVolume &&
		mount.VolumeOptions != nil &&
		mount.VolumeOptions.DriverConfig != nil &&
		mount.VolumeOptions.DriverConfig.Name != "" &&
		mount.VolumeOptions.DriverConfig.Name != "local"

}

// SetTask returns true when the filter is enabled for a given task.
func (f *PluginFilter) SetTask(t *api.Task) bool {
	c := t.Spec.GetContainer()

	var volumeTemplates bool
	if c != nil {
		for _, mount := range c.Mounts {
			if referencesVolumePlugin(mount) {
				volumeTemplates = true
				break
			}
		}
	}

	if (c != nil && volumeTemplates) || len(t.Networks) > 0 {
		f.t = t
		return true
	}

	return false
}

// Check returns true if the task can be scheduled into the given node.
// TODO(amitshukla): investigate storing Plugins as a map so it can be easily probed
func (f *PluginFilter) Check(n *NodeInfo) bool {
	// Get list of plugins on the node
	nodePlugins := n.Description.Engine.Plugins

	// Check if all volume plugins required by task are installed on node
	container := f.t.Spec.GetContainer()
	if container != nil {
		for _, mount := range container.Mounts {
			if referencesVolumePlugin(mount) {
				if !f.pluginExistsOnNode("Volume", mount.VolumeOptions.DriverConfig.Name, nodePlugins) {
					return false
				}
			}
		}
	}

	// Check if all network plugins required by task are installed on node
	for _, tn := range f.t.Networks {
		if tn.Network != nil && tn.Network.DriverState != nil && tn.Network.DriverState.Name != "" {
			if !f.pluginExistsOnNode("Network", tn.Network.DriverState.Name, nodePlugins) {
				return false
			}
		}
	}
	return true
}

// pluginExistsOnNode returns true if the (pluginName, pluginType) pair is present in nodePlugins
func (f *PluginFilter) pluginExistsOnNode(pluginType string, pluginName string, nodePlugins []api.PluginDescription) bool {
	for _, np := range nodePlugins {
		if pluginType != np.Type {
			continue
		}
		if pluginName == np.Name {
			return true
		}
		// This does not use the reference package to avoid the
		// overhead of parsing references as part of the scheduling
		// loop. This is okay only because plugin names are a very
		// strict subset of the reference grammar that is always
		// name:tag.
		if strings.HasPrefix(np.Name, pluginName) && np.Name[len(pluginName):] == ":latest" {
			return true
		}
	}
	return false
}

// Explain returns an explanation of a failure.
func (f *PluginFilter) Explain(nodes int) string {
	if nodes == 1 {
		return "missing plugin on 1 node"
	}
	return fmt.Sprintf("missing plugin on %d nodes", nodes)
}

// ConstraintFilter selects only nodes that match certain labels.
type ConstraintFilter struct {
	constraints []constraint.Constraint
}

// SetTask returns true when the filter is enable for a given task.
func (f *ConstraintFilter) SetTask(t *api.Task) bool {
	if t.Spec.Placement == nil || len(t.Spec.Placement.Constraints) == 0 {
		return false
	}

	constraints, err := constraint.Parse(t.Spec.Placement.Constraints)
	if err != nil {
		// constraints have been validated at controlapi
		// if in any case it finds an error here, treat this task
		// as constraint filter disabled.
		return false
	}
	f.constraints = constraints
	return true
}

// Check returns true if the task's constraint is supported by the given node.
func (f *ConstraintFilter) Check(n *NodeInfo) bool {
	return constraint.NodeMatches(f.constraints, n.Node)
}

// Explain returns an explanation of a failure.
func (f *ConstraintFilter) Explain(nodes int) string {
	if nodes == 1 {
		return "scheduling constraints not satisfied on 1 node"
	}
	return fmt.Sprintf("scheduling constraints not satisfied on %d nodes", nodes)
}

// PlatformFilter selects only nodes that run the required platform.
type PlatformFilter struct {
	supportedPlatforms []*api.Platform
}

// SetTask returns true when the filter is enabled for a given task.
func (f *PlatformFilter) SetTask(t *api.Task) bool {
	placement := t.Spec.Placement
	if placement != nil {
		// copy the platform information
		f.supportedPlatforms = placement.Platforms
		if len(placement.Platforms) > 0 {
			return true
		}
	}
	return false
}

// Check returns true if the task can be scheduled into the given node.
func (f *PlatformFilter) Check(n *NodeInfo) bool {
	// if the supportedPlatforms field is empty, then either it wasn't
	// provided or there are no constraints
	if len(f.supportedPlatforms) == 0 {
		return true
	}
	// check if the platform for the node is supported
	nodePlatform := n.Description.Platform
	for _, p := range f.supportedPlatforms {
		if (p.Architecture == "" || p.Architecture == nodePlatform.Architecture) && (p.OS == "" || p.OS == nodePlatform.OS) {
			return true
		}
	}
	return false
}

// Explain returns an explanation of a failure.
func (f *PlatformFilter) Explain(nodes int) string {
	if nodes == 1 {
		return "unsupported platform on 1 node"
	}
	return fmt.Sprintf("unsupported platform on %d nodes", nodes)
}
