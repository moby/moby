package container

import "github.com/docker/docker/api/types/strslice"

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

// WithCmd sets the comannds of the container
func WithCmd(cmds ...string) func(*TestContainerConfig) {
	return func(c *TestContainerConfig) {
		c.Config.Cmd = strslice.StrSlice(cmds)
	}
}
