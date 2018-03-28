package container

import (
	"fmt"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
)

// WithName sets the name of the container
func WithName(name string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Name = name
	}
}

// WithLinks sets the links of the container
func WithLinks(links ...string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.HostConfig.Links = links
	}
}

// WithImage sets the image of the container
func WithImage(image string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.Image = image
	}
}

// WithCmd sets the comannds of the container
func WithCmd(cmds ...string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.Cmd = strslice.StrSlice(cmds)
	}
}

// WithNetworkMode sets the network mode of the container
func WithNetworkMode(mode string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.HostConfig.NetworkMode = containertypes.NetworkMode(mode)
	}
}

// WithExposedPorts sets the exposed ports of the container
func WithExposedPorts(ports ...string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.ExposedPorts = map[nat.Port]struct{}{}
		for _, port := range ports {
			c.Config.ExposedPorts[nat.Port(port)] = struct{}{}
		}
	}
}

// WithTty sets the TTY mode of the container
func WithTty(tty bool) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.Tty = tty
	}
}

// WithWorkingDir sets the working dir of the container
func WithWorkingDir(dir string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.WorkingDir = dir
	}
}

// WithVolume sets the volume of the container
func WithVolume(name string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		if c.Config.Volumes == nil {
			c.Config.Volumes = map[string]struct{}{}
		}
		c.Config.Volumes[name] = struct{}{}
	}
}

// WithBind sets the bind mount of the container
func WithBind(src, target string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.HostConfig.Binds = append(c.HostConfig.Binds, fmt.Sprintf("%s:%s", src, target))
	}
}

// WithIPv4 sets the specified ip for the specified network of the container
func WithIPv4(network, ip string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		if v, ok := c.NetworkingConfig.EndpointsConfig[network]; !ok || v == nil {
			c.NetworkingConfig.EndpointsConfig[network] = &networktypes.EndpointSettings{}
		}
		if c.NetworkingConfig.EndpointsConfig[network].IPAMConfig == nil {
			c.NetworkingConfig.EndpointsConfig[network].IPAMConfig = &networktypes.EndpointIPAMConfig{}
		}
		c.NetworkingConfig.EndpointsConfig[network].IPAMConfig.IPv4Address = ip
	}
}

// WithIPv6 sets the specified ip6 for the specified network of the container
func WithIPv6(network, ip string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		if v, ok := c.NetworkingConfig.EndpointsConfig[network]; !ok || v == nil {
			c.NetworkingConfig.EndpointsConfig[network] = &networktypes.EndpointSettings{}
		}
		if c.NetworkingConfig.EndpointsConfig[network].IPAMConfig == nil {
			c.NetworkingConfig.EndpointsConfig[network].IPAMConfig = &networktypes.EndpointIPAMConfig{}
		}
		c.NetworkingConfig.EndpointsConfig[network].IPAMConfig.IPv6Address = ip
	}
}
