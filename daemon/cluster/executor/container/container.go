package container

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	enginecontainer "github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
)

const (
	// Explictly use the kernel's default setting for CPU quota of 100ms.
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
	container := t.Spec.GetContainer()
	if container == nil {
		return exec.ErrRuntimeUnsupported
	}

	if container.Image == "" {
		return ErrImageRequired
	}

	// index the networks by name
	c.networksAttachments = make(map[string]*api.NetworkAttachment, len(t.Networks))
	for _, attachment := range t.Networks {
		c.networksAttachments[attachment.Network.Spec.Annotations.Name] = attachment
	}

	c.task = t
	return nil
}

func (c *containerConfig) endpoint() *api.Endpoint {
	return c.task.Endpoint
}

func (c *containerConfig) spec() *api.ContainerSpec {
	return c.task.Spec.GetContainer()
}

func (c *containerConfig) name() string {
	if c.task.Annotations.Name != "" {
		// if set, use the container Annotations.Name field, set in the orchestrator.
		return c.task.Annotations.Name
	}

	// fallback to service.instance.id.
	return strings.Join([]string{c.task.ServiceAnnotations.Name, fmt.Sprint(c.task.Instance), c.task.ID}, ".")
}

func (c *containerConfig) image() string {
	raw := c.spec().Image
	ref, err := reference.ParseNamed(raw)
	if err != nil {
		return raw
	}
	return reference.WithDefaultTag(ref).String()
}

func (c *containerConfig) volumes() map[string]struct{} {
	r := make(map[string]struct{})

	for _, mount := range c.spec().Mounts {
		// pick off all the volume mounts.
		if mount.Type != api.MountTypeVolume {
			continue
		}

		var (
			name string
			mask = getMountMask(mount)
		)

		if mount.Template != nil {
			name = mount.Template.Annotations.Name
		}

		if name != "" {
			r[fmt.Sprintf("%s:%s:%s", name, mount.Target, mask)] = struct{}{}
		} else {
			r[fmt.Sprintf("%s:%s", mount.Target, mask)] = struct{}{}
		}
	}

	return r
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
		config.Entrypoint = append(config.Entrypoint, c.spec().Command[0])
		config.Cmd = append(config.Cmd, c.spec().Command[1:]...)
	} else if len(c.spec().Args) > 0 {
		// In this case, we assume the image has an Entrypoint and Args
		// specifies the arguments for that entrypoint.
		config.Cmd = c.spec().Args
	}

	return config
}

func (c *containerConfig) labels() map[string]string {
	var (
		system = map[string]string{
			"task":         "", // mark as cluster task
			"task.id":      c.task.ID,
			"task.name":    fmt.Sprintf("%v.%v", c.task.ServiceAnnotations.Name, c.task.Instance),
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

func (c *containerConfig) bindMounts() []string {
	var r []string

	for _, val := range c.spec().Mounts {
		mask := getMountMask(val)
		if val.Type == api.MountTypeBind {
			r = append(r, fmt.Sprintf("%s:%s:%s", val.Source, val.Target, mask))
		}
	}

	return r
}

func getMountMask(m *api.Mount) string {
	maskOpts := []string{"ro"}
	if m.Writable {
		maskOpts[0] = "rw"
	}

	switch m.Propagation {
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

	if !m.Populate {
		maskOpts = append(maskOpts, "nocopy")
	}

	switch m.Mcsaccessmode {
	case api.MountMCSAccessModeShared:
		maskOpts = append(maskOpts, "z")
	case api.MountMCSAccessModePrivate:
		maskOpts = append(maskOpts, "Z")
	}

	return strings.Join(maskOpts, ",")
}

func (c *containerConfig) hostConfig() *enginecontainer.HostConfig {
	return &enginecontainer.HostConfig{
		Resources: c.resources(),
		Binds:     c.bindMounts(),
	}
}

// This handles the case of volumes that are defined inside a service Mount
func (c *containerConfig) volumeCreateRequest(mount *api.Mount) *types.VolumeCreateRequest {
	return &types.VolumeCreateRequest{
		Name:       mount.Template.Annotations.Name,
		Driver:     mount.Template.DriverConfig.Name,
		DriverOpts: mount.Template.DriverConfig.Options,
	}
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
	if c.task.Spec.GetContainer() != nil {
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

	log.Printf("Createing service config in agent for t = %+v", c.task)
	svcCfg := &clustertypes.ServiceConfig{
		Name:             c.task.ServiceAnnotations.Name,
		ID:               c.task.ServiceID,
		VirtualAddresses: make(map[string]*clustertypes.VirtualAddress),
	}

	for _, na := range c.task.Networks {
		svcCfg.VirtualAddresses[na.Network.ID] = &clustertypes.VirtualAddress{
			// We support only IPv4 virtual IP for now.
			IPv4: c.virtualIP(na.Network.ID),
		}
	}

	if c.task.Endpoint != nil {
		for _, ePort := range c.task.Endpoint.ExposedPorts {
			svcCfg.ExposedPorts = append(svcCfg.ExposedPorts, &clustertypes.PortConfig{
				Name:     ePort.Name,
				Protocol: int32(ePort.Protocol),
				Port:     ePort.Port,
				NodePort: ePort.SwarmPort,
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
		IPAM: network.IPAM{
			Driver: na.Network.IPAM.Driver.Name,
		},
		Options:        na.Network.DriverState.Options,
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
