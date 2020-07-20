package executor

import (
	"context"
	"io"
	"net"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/solver/pb"
)

type Meta struct {
	Args           []string
	Env            []string
	User           string
	Cwd            string
	Tty            bool
	ReadonlyRootFS bool
	ExtraHosts     []HostIP
	NetMode        pb.NetMode
	SecurityMode   pb.SecurityMode
}

type Mount struct {
	Src      cache.Mountable
	Selector string
	Dest     string
	Readonly bool
}

type ProcessInfo struct {
	Meta           Meta
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
}

type Executor interface {
	// Run will start a container for the given process with rootfs, mounts.
	// `id` is an optional name for the container so it can be referenced later via Exec.
	// `started` is an optional channel that will be closed when the container setup completes and has started running.
	Run(ctx context.Context, id string, rootfs cache.Mountable, mounts []Mount, process ProcessInfo, started chan<- struct{}) error
	// Exec will start a process in container matching `id`. An error will be returned
	// if the container failed to start (via Run) or has exited before Exec is called.
	Exec(ctx context.Context, id string, process ProcessInfo) error
}

type HostIP struct {
	Host string
	IP   net.IP
}
