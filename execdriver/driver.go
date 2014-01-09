package execdriver

import (
	"io"
	"net"
)

// Network settings of the container
type Network struct {
	Gateway     string
	IPAddress   net.IPAddr
	IPPrefixLen int
	Mtu         int
}

// Container / Process / Whatever, we can redefine the conatiner here
// to be what it should be and not have to carry the baggage of the
// container type in the core with backward compat.  This is what a
// driver needs to execute a process inside of a conatiner.  This is what
// a container is at it's core.
type Container struct {
	Name        string // unique name for the conatienr
	Privileged  bool
	User        string
	Dir         string // root fs of the container
	InitPath    string // dockerinit
	Entrypoint  string
	Args        []string
	Environment map[string]string
	WorkingDir  string
	Network     *Network // if network is nil then networking is disabled
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer

	Context interface{}
}

// State can be handled internally in the drivers
type Driver interface {
	Start(c *Container) error
	Stop(c *Container) error
	Kill(c *Container, sig int) error
	Running(c *Container) (bool, error)
	Wait(c *Container, seconds int) error
}
