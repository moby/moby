package container

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/docker/reference"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
)

const (
	// Explicitly use the kernel's default setting for CPU quota of 100ms.
	// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
	cpuQuotaPeriod = 100 * time.Millisecond

	// systemLabelPrefix represents the reserved namespace for system labels.
	systemLabelPrefix = "com.docker.swarm"
)

// containerConfig converts task properties into docker container compatible
// components.
type containerConfig struct {
	task                *api.Task
	networksAttachments map[string]*api.NetworkAttachment
}

// newContainerConfig returns a validated container config. No methods should
// return an error if this function returns without error.
func newContainerConfig(t *api.Task) (*containerConfig, error) {
	var c containerConfig
	return &c, c.setTask(t)
}

func (c *containerConfig) setTask(t *api.Task) error {
	if t.Spec.GetContainer() == nil && t.Spec.GetAttachment() == nil {
		return exec.ErrRuntimeUnsupported
	}

	container := t.Spec.GetContainer()
	if container != nil {
		if container.Image == "" {
			return ErrImageRequired
		}

		if err := validateMounts(container.Mounts); err != nil {
			return err
		}
	}

	// index the networks by name
	c.networksAttachments = make(map[string]*api.NetworkAttachment, len(t.Networks))
	for _, attachment := range t.Networks {
		c.networksAttachments[attachment.Network.Spec.Annotations.Name] = attachment
	}

	c.task = t
	return nil
}

func (c *containerConfig) id() string {
	attachment := c.task.Spec.GetAttachment()
	if attachment == nil {
		return ""
	}

	return attachment.ContainerID
}

func (c *containerConfig) taskID() string {
	return c.task.ID
}

func (c *containerConfig) endpoint() *api.Endpoint {
	return c.task.Endpoint
}

func (c *containerConfig) spec() *api.ContainerSpec {
	return c.task.Spec.GetContainer()
}

func (c *containerConfig) nameOrID() string {
	if c.task.Spec.GetContainer() != nil {
		return c.name()
	}

	return c.id()
}

func (c *containerConfig) name() string {
	if c.task.Annotations.Name != "" {
		// if set, use the container Annotations.Name field, set in the orchestrator.
		return c.task.Annotations.Name
	}

	// fallback to service.slot.id.
	return strings.Join([]string{c.task.ServiceAnnotations.Name, fmt.Sprint(c.task.Slot), c.task.ID}, ".")
}

func (c *containerConfig) image() string {
	raw := c.spec().Image
	ref, err := reference.ParseNamed(raw)
	if err != nil {
		return raw
	}
	return reference.WithDefaultTag(ref).String()
}

func (c *containerConfig) config() *enginecontainer.Config {
	config := &enginecontainer.Config{
		Labels:     c.labels(),
		User:       c.spec().User,
		Env:        c.spec().Env,
		WorkingDir: c.spec().Dir,
		Image:      c.image(),
		Volumes:    c.volumes(),
	}

	if len(c.spec().Command) > 0 {
		// If Command is provided, we replace the whole invocation with Command
		// by replacing Entrypoint and specifying Cmd. Args is ignored in this
		// case.
		config.Entrypoint = append(config.Entrypoint, c.spec().Command...)
		config.Cmd = append(config.Cmd, c.spec().Args...)
	} else if len(c.spec().Args) > 0 {
		// In this case, we assume the image has an Entrypoint and Args
		// specifies the arguments for that entrypoint.
		config.Cmd = c.spec().Args
	}

	return config
}

func (c *containerConfig) labels() map[string]string {
	taskName := c.task.Annotations.Name
	if taskName == "" {
		if c.task.Slot != 0 {
			taskName = fmt.Sprintf("%v.%v.%v", c.task.ServiceAnnotations.Name, c.task.Slot, c.task.ID)
		} else {
			taskName = fmt.Sprintf("%v.%v.%v", c.task.ServiceAnnotations.Name, c.task.NodeID, c.task.ID)
		}
	}
	var (
		system = map[string]string{
			"task":         "", // mark as cluster task
			"task.id":      c.task.ID,
			"task.name":    taskName,
			"node.id":      c.task.NodeID,
			"service.id":   c.task.ServiceID,
			"service.name": c.task.ServiceAnnotations.Name,
		}
		labels = make(map[string]string)
	)

	// base labels are those defined in the spec.
	for k, v := range c.spec().Labels {
		labels[k] = v
	}

	// we then apply the overrides from the task, which may be set via the
	// orchestrator.
	for k, v := range c.task.Annotations.Labels {
		labels[k] = v
	}

	// finally, we apply the system labels, which override all labels.
	for k, v := range system {
		labels[strings.Join([]string{systemLabelPrefix, k}, ".")] = v
	}

	return labels
}

// volumes gets placed into the Volumes field on the containerConfig.
func (c *containerConfig) volumes() map[string]struct{} {
	r := make(map[string]struct{})
	// Volumes *only* creates anonymous volumes. The rest is mixed in with
	// binds, which aren't actually binds. Basically, any volume that
	// results in a single component must be added here.
	//
	// This is reversed engineered from the behavior of the engine API.
	for _, mount := range c.spec().Mounts {
		if mount.Type == api.MountTypeVolume && mount.Source == "" {
			r[mount.Target] = struct{}{}
		}
	}
	return r
}

func (c *containerConfig) tmpfs() map[string]string {
	r := make(map[string]string)

	for _, spec := range c.spec().Mounts {
		if spec.Type != api.MountTypeTmpfs {
			continue
		}

		r[spec.Target] = getMountMask(&spec)
	}

	return r
}

func (c *containerConfig) binds() []string {
	var r []string
	for _, mount := range c.spec().Mounts {
		if mount.Type == api.MountTypeBind || (mount.Type == api.MountTypeVolume && mount.Source != "") {
			spec := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
			mask := getMountMask(&mount)
			if mask != "" {
				spec = fmt.Sprintf("%s:%s", spec, mask)
			}
			r = append(r, spec)
		}
	}
	return r
}

func getMountMask(m *api.Mount) string {
	var maskOpts []string
	if m.ReadOnly {
		maskOpts = append(maskOpts, "ro")
	}

	switch m.Type {
	case api.MountTypeVolume:
		if m.VolumeOptions != nil && m.VolumeOptions.NoCopy {
			maskOpts = append(maskOpts, "nocopy")
		}
	case api.MountTypeBind:
		if m.BindOptions == nil {
			break
		}

		switch m.BindOptions.Propagation {
		case api.MountPropagationPrivate:
			maskOpts = append(maskOpts, "private")
		case api.MountPropagationRPrivate:
			maskOpts = append(maskOpts, "rprivate")
		case api.MountPropagationShared:
			maskOpts = append(maskOpts, "shared")
		case api.MountPropagationRShared:
			maskOpts = append(maskOpts, "rshared")
		case api.MountPropagationSlave:
			maskOpts = append(maskOpts, "slave")
		case api.MountPropagationRSlave:
			maskOpts = append(maskOpts, "rslave")
		}
	case api.MountTypeTmpfs:
		if m.TmpfsOptions == nil {
			break
		}

		if m.TmpfsOptions.Mode != 0 {
			maskOpts = append(maskOpts, fmt.Sprintf("mode=%o", m.TmpfsOptions.Mode))
		}

		if m.TmpfsOptions.SizeBytes != 0 {
			// calculate suffix here, making this linux specific, but that is
			// okay, since API is that way anyways.

			// we do this by finding the suffix that divides evenly into the
			// value, returing the value itself, with no suffix, if it fails.
			//
			// For the most part, we don't enforce any semantic to this values.
			// The operating system will usually align this and enforce minimum
			// and maximums.
			var (
				size   = m.TmpfsOptions.SizeBytes
				suffix string
			)
			for _, r := range []struct {
				suffix  string
				divisor int64
			}{
				{"g", 1 << 30},
				{"m", 1 << 20},
				{"k", 1 << 10},
			} {
				if size%r.divisor == 0 {
					size = size / r.divisor
					suffix = r.suffix
					break
				}
			}

			maskOpts = append(maskOpts, fmt.Sprintf("size=%d%s", size, suffix))
		}
	}

	return strings.Join(maskOpts, ",")
}

func (c *containerConfig) hostConfig() *enginecontainer.HostConfig {
	hc := &enginecontainer.HostConfig{
		Resources: c.resources(),
		Binds:     c.binds(),
		Tmpfs:     c.tmpfs(),
		GroupAdd:  c.spec().Groups,
	}

	if c.task.LogDriver != nil {
		hc.LogConfig = enginecontainer.LogConfig{
			Type:   c.task.LogDriver.Name,
			Config: c.task.LogDriver.Options,
		}
	}

	return hc
}

// This handles the case of volumes that are defined inside a service Mount
func (c *containerConfig) volumeCreateRequest(mount *api.Mount) *types.VolumeCreateRequest {
	var (
		driverName string
		driverOpts map[string]string
		labels     map[string]string
	)

	if mount.VolumeOptions != nil && mount.VolumeOptions.DriverConfig != nil {
		driverName = mount.VolumeOptions.DriverConfig.Name
		driverOpts = mount.VolumeOptions.DriverConfig.Options
		labels = mount.VolumeOptions.Labels
	}

	if mount.VolumeOptions != nil {
		return &types.VolumeCreateRequest{
			Name:       mount.Source,
			Driver:     driverName,
			DriverOpts: driverOpts,
			Labels:     labels,
		}
	}
	return nil
}

func (c *containerConfig) resources() enginecontainer.Resources {
	resources := enginecontainer.Resources{}

	// If no limits are specified let the engine use its defaults.
	//
	// TODO(aluzzardi): We might want to set some limits anyway otherwise
	// "unlimited" tasks will step over the reservation of other tasks.
	r := c.task.Spec.Resources
	if r == nil || r.Limits == nil {
		return resources
	}

	if r.Limits.MemoryBytes > 0 {
		resources.Memory = r.Limits.MemoryBytes
	}

	if r.Limits.NanoCPUs > 0 {
		// CPU Period must be set in microseconds.
		resources.CPUPeriod = int64(cpuQuotaPeriod / time.Microsecond)
		resources.CPUQuota = r.Limits.NanoCPUs * resources.CPUPeriod / 1e9
	}

	return resources
}

// Docker daemon supports just 1 network during container create.
func (c *containerConfig) createNetworkingConfig() *network.NetworkingConfig {
	var networks []*api.NetworkAttachment
	if c.task.Spec.GetContainer() != nil || c.task.Spec.GetAttachment() != nil {
		networks = c.task.Networks
	}

	epConfig := make(map[string]*network.EndpointSettings)
	if len(networks) > 0 {
		epConfig[networks[0].Network.Spec.Annotations.Name] = getEndpointConfig(networks[0])
	}

	return &network.NetworkingConfig{EndpointsConfig: epConfig}
}

// TODO: Merge this function with createNetworkingConfig after daemon supports multiple networks in container create
func (c *containerConfig) connectNetworkingConfig() *network.NetworkingConfig {
	var networks []*api.NetworkAttachment
	if c.task.Spec.GetContainer() != nil {
		networks = c.task.Networks
	}

	// First network is used during container create. Other networks are used in "docker network connect"
	if len(networks) < 2 {
		return nil
	}

	epConfig := make(map[string]*network.EndpointSettings)
	for _, na := range networks[1:] {
		epConfig[na.Network.Spec.Annotations.Name] = getEndpointConfig(na)
	}
	return &network.NetworkingConfig{EndpointsConfig: epConfig}
}

func getEndpointConfig(na *api.NetworkAttachment) *network.EndpointSettings {
	var ipv4, ipv6 string
	for _, addr := range na.Addresses {
		ip, _, err := net.ParseCIDR(addr)
		if err != nil {
			continue
		}

		if ip.To4() != nil {
			ipv4 = ip.String()
			continue
		}

		if ip.To16() != nil {
			ipv6 = ip.String()
		}
	}

	return &network.EndpointSettings{
		NetworkID: na.Network.ID,
		IPAMConfig: &network.EndpointIPAMConfig{
			IPv4Address: ipv4,
			IPv6Address: ipv6,
		},
	}
}

func (c *containerConfig) virtualIP(networkID string) string {
	if c.task.Endpoint == nil {
		return ""
	}

	for _, eVip := range c.task.Endpoint.VirtualIPs {
		// We only support IPv4 VIPs for now.
		if eVip.NetworkID == networkID {
			vip, _, err := net.ParseCIDR(eVip.Addr)
			if err != nil {
				return ""
			}

			return vip.String()
		}
	}

	return ""
}

func (c *containerConfig) serviceConfig() *clustertypes.ServiceConfig {
	if len(c.task.Networks) == 0 {
		return nil
	}

	logrus.Debugf("Creating service config in agent for t = %+v", c.task)
	svcCfg := &clustertypes.ServiceConfig{
		Name:             c.task.ServiceAnnotations.Name,
		Aliases:          make(map[string][]string),
		ID:               c.task.ServiceID,
		VirtualAddresses: make(map[string]*clustertypes.VirtualAddress),
	}

	for _, na := range c.task.Networks {
		svcCfg.VirtualAddresses[na.Network.ID] = &clustertypes.VirtualAddress{
			// We support only IPv4 virtual IP for now.
			IPv4: c.virtualIP(na.Network.ID),
		}
		if len(na.Aliases) > 0 {
			svcCfg.Aliases[na.Network.ID] = na.Aliases
		}
	}

	if c.task.Endpoint != nil {
		for _, ePort := range c.task.Endpoint.Ports {
			svcCfg.ExposedPorts = append(svcCfg.ExposedPorts, &clustertypes.PortConfig{
				Name:          ePort.Name,
				Protocol:      int32(ePort.Protocol),
				TargetPort:    ePort.TargetPort,
				PublishedPort: ePort.PublishedPort,
			})
		}
	}

	return svcCfg
}

// networks returns a list of network names attached to the container. The
// returned name can be used to lookup the corresponding network create
// options.
func (c *containerConfig) networks() []string {
	var networks []string

	for name := range c.networksAttachments {
		networks = append(networks, name)
	}

	return networks
}

func (c *containerConfig) networkCreateRequest(name string) (clustertypes.NetworkCreateRequest, error) {
	na, ok := c.networksAttachments[name]
	if !ok {
		return clustertypes.NetworkCreateRequest{}, errors.New("container: unknown network referenced")
	}

	options := types.NetworkCreate{
		// ID:     na.Network.ID,
		Driver: na.Network.DriverState.Name,
		IPAM: &network.IPAM{
			Driver: na.Network.IPAM.Driver.Name,
		},
		Options:        na.Network.DriverState.Options,
		Labels:         na.Network.Spec.Annotations.Labels,
		Internal:       na.Network.Spec.Internal,
		EnableIPv6:     na.Network.Spec.Ipv6Enabled,
		CheckDuplicate: true,
	}

	for _, ic := range na.Network.IPAM.Configs {
		c := network.IPAMConfig{
			Subnet:  ic.Subnet,
			IPRange: ic.Range,
			Gateway: ic.Gateway,
		}
		options.IPAM.Config = append(options.IPAM.Config, c)
	}

	return clustertypes.NetworkCreateRequest{na.Network.ID, types.NetworkCreateRequest{Name: name, NetworkCreate: options}}, nil
}

func (c containerConfig) eventFilter() filters.Args {
	filter := filters.NewArgs()
	filter.Add("type", events.ContainerEventType)
	filter.Add("name", c.name())
	filter.Add("label", fmt.Sprintf("%v.task.id=%v", systemLabelPrefix, c.task.ID))
	return filter
}
