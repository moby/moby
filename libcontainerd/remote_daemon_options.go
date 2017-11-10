// +build !windows

package libcontainerd

import "fmt"

// WithRemoteAddr sets the external containerd socket to connect to.
func WithRemoteAddr(addr string) RemoteOption {
	return rpcAddr(addr)
}

type rpcAddr string

func (a rpcAddr) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.GRPC.Address = string(a)
		return nil
	}
	return fmt.Errorf("WithRemoteAddr option not supported for this remote")
}

// WithRemoteAddrUser sets the uid and gid to create the RPC address with
func WithRemoteAddrUser(uid, gid int) RemoteOption {
	return rpcUser{uid, gid}
}

type rpcUser struct {
	uid int
	gid int
}

func (u rpcUser) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.GRPC.UID = u.uid
		remote.GRPC.GID = u.gid
		return nil
	}
	return fmt.Errorf("WithRemoteAddr option not supported for this remote")
}

// WithStartDaemon defines if libcontainerd should also run containerd daemon.
func WithStartDaemon(start bool) RemoteOption {
	return startDaemon(start)
}

type startDaemon bool

func (s startDaemon) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.startDaemon = bool(s)
		return nil
	}
	return fmt.Errorf("WithStartDaemon option not supported for this remote")
}

// WithLogLevel defines which log level to starts containerd with.
// This only makes sense if WithStartDaemon() was set to true.
func WithLogLevel(lvl string) RemoteOption {
	return logLevel(lvl)
}

type logLevel string

func (l logLevel) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.Debug.Level = string(l)
		return nil
	}
	return fmt.Errorf("WithDebugLog option not supported for this remote")
}

// WithDebugAddress defines at which location the debug GRPC connection
// should be made
func WithDebugAddress(addr string) RemoteOption {
	return debugAddress(addr)
}

type debugAddress string

func (d debugAddress) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.Debug.Address = string(d)
		return nil
	}
	return fmt.Errorf("WithDebugAddress option not supported for this remote")
}

// WithMetricsAddress defines at which location the debug GRPC connection
// should be made
func WithMetricsAddress(addr string) RemoteOption {
	return metricsAddress(addr)
}

type metricsAddress string

func (m metricsAddress) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.Metrics.Address = string(m)
		return nil
	}
	return fmt.Errorf("WithMetricsAddress option not supported for this remote")
}

// WithSnapshotter defines snapshotter driver should be used
func WithSnapshotter(name string) RemoteOption {
	return snapshotter(name)
}

type snapshotter string

func (s snapshotter) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.snapshotter = string(s)
		return nil
	}
	return fmt.Errorf("WithSnapshotter option not supported for this remote")
}

// WithPlugin allow configuring a containerd plugin
// configuration values passed needs to be quoted if quotes are needed in
// the toml format.
func WithPlugin(name string, conf interface{}) RemoteOption {
	return pluginConf{
		name: name,
		conf: conf,
	}
}

type pluginConf struct {
	// Name is the name of the plugin
	name string
	conf interface{}
}

func (p pluginConf) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.pluginConfs.Plugins[p.name] = p.conf
		return nil
	}
	return fmt.Errorf("WithPlugin option not supported for this remote")
}
