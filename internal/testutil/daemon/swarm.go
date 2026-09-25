package daemon

import (
	"context"
	"fmt"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

const (
	// DefaultSwarmPort is the default port use for swarm in the tests
	DefaultSwarmPort       = 2477
	defaultSwarmListenAddr = "0.0.0.0"
)

var (
	startArgsWithIptables = []string{"--swarm-default-advertise-addr=lo"}
	startArgs             = []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
)

// StartNode (re)starts the daemon
func (d *Daemon) StartNode(t testing.TB) {
	t.Helper()
	d.Start(t, d.startArgs()...)
}

// StartNodeWithBusybox starts daemon to be used as a swarm node, and loads the busybox image
func (d *Daemon) StartNodeWithBusybox(ctx context.Context, t testing.TB) {
	t.Helper()
	d.StartWithBusybox(ctx, t, d.startArgs()...)
}

// RestartNode restarts a daemon to be used as a swarm node
func (d *Daemon) RestartNode(t testing.TB) {
	t.Helper()
	// avoid networking conflicts
	d.Stop(t)
	d.Start(t, d.startArgs()...)
}

// StartAndSwarmInit starts the daemon (with busybox) and init the swarm
func (d *Daemon) StartAndSwarmInit(ctx context.Context, t testing.TB) {
	d.StartNodeWithBusybox(ctx, t)
	var req swarm.InitRequest
	if d.swarmListenAddr != defaultSwarmListenAddr {
		req.AdvertiseAddr = d.swarmListenAddr
	}
	d.SwarmInit(ctx, t, req)
}

// StartAndSwarmJoin starts the daemon (with busybox) and join the specified swarm as worker or manager
func (d *Daemon) StartAndSwarmJoin(ctx context.Context, t testing.TB, leader *Daemon, manager bool) {
	t.Helper()
	d.StartNodeWithBusybox(ctx, t)

	tokens := leader.JoinTokens(t)
	token := tokens.Worker
	if manager {
		token = tokens.Manager
	}
	t.Logf("[%s] joining swarm manager [%s]@%s, swarm listen addr %s", d.id, leader.id, leader.SwarmListenAddr(), d.SwarmListenAddr())
	req := swarm.JoinRequest{
		RemoteAddrs: []string{leader.SwarmListenAddr()},
		JoinToken:   token,
	}
	if d.swarmListenAddr != defaultSwarmListenAddr {
		req.AdvertiseAddr = d.swarmListenAddr
	}
	d.SwarmJoin(ctx, t, req)
}

// SpecConstructor defines a swarm spec constructor
type SpecConstructor func(*swarm.Spec)

// SwarmListenAddr returns the listen-addr used for the daemon
func (d *Daemon) SwarmListenAddr() string {
	return fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
}

// NodeID returns the swarm mode node ID
func (d *Daemon) NodeID() string {
	return d.CachedInfo.Swarm.NodeID
}

// SwarmInitWithError initializes a new swarm cluster and returns an error.
func (d *Daemon) SwarmInitWithError(ctx context.Context, t testing.TB, req swarm.InitRequest) error {
	t.Helper()
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	if req.DefaultAddrPool == nil {
		req.DefaultAddrPool = d.DefaultAddrPool
		req.SubnetSize = d.SubnetSize
	}
	if d.DataPathPort > 0 {
		req.DataPathPort = d.DataPathPort
	}
	cli := d.NewClientT(t)
	defer cli.Close()
	_, err := cli.SwarmInit(ctx, client.SwarmInitOptions{
		ListenAddr:       req.ListenAddr,
		AdvertiseAddr:    req.AdvertiseAddr,
		DataPathAddr:     req.DataPathAddr,
		DataPathPort:     req.DataPathPort,
		ForceNewCluster:  req.ForceNewCluster,
		Spec:             req.Spec,
		AutoLockManagers: req.AutoLockManagers,
		Availability:     req.Availability,
		DefaultAddrPool:  req.DefaultAddrPool,
		SubnetSize:       req.SubnetSize,
	})
	if err == nil {
		d.CachedInfo = d.Info(t)
	}
	return err
}

// SwarmInit initializes a new swarm cluster.
func (d *Daemon) SwarmInit(ctx context.Context, t testing.TB, req swarm.InitRequest) {
	t.Helper()
	err := d.SwarmInitWithError(ctx, t, req)
	assert.NilError(t, err, "initializing swarm")
}

// SwarmJoin joins a daemon to an existing cluster.
func (d *Daemon) SwarmJoin(ctx context.Context, t testing.TB, req swarm.JoinRequest) {
	t.Helper()
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	cli := d.NewClientT(t)
	defer cli.Close()
	_, err := cli.SwarmJoin(ctx, client.SwarmJoinOptions{
		ListenAddr:    req.ListenAddr,
		AdvertiseAddr: req.AdvertiseAddr,
		DataPathAddr:  req.DataPathAddr,
		RemoteAddrs:   req.RemoteAddrs,
		JoinToken:     req.JoinToken,
		Availability:  req.Availability,
	})
	assert.NilError(t, err, "[%s] joining swarm", d.id)
	d.CachedInfo = d.Info(t)
}

// SwarmLeave forces daemon to leave current cluster.
//
// The passed in testing.TB is only used to validate that the client was successfully created
// Some tests rely on error checking the result of the actual unlock, so allow
// the error to be returned.
func (d *Daemon) SwarmLeave(ctx context.Context, t testing.TB, force bool) error {
	cli := d.NewClientT(t)
	defer cli.Close()
	_, err := cli.SwarmLeave(ctx, client.SwarmLeaveOptions{Force: force})
	return err
}

// SwarmInfo returns the swarm information of the daemon
func (d *Daemon) SwarmInfo(ctx context.Context, t testing.TB) swarm.Info {
	t.Helper()
	cli := d.NewClientT(t)
	result, err := cli.Info(ctx, client.InfoOptions{})
	assert.NilError(t, err, "get swarm info")
	info := result.Info
	return info.Swarm
}

// SwarmUnlock tries to unlock a locked swarm
//
// The passed in testing.TB is only used to validate that the client was successfully created
// Some tests rely on error checking the result of the actual unlock, so allow
// the error to be returned.
func (d *Daemon) SwarmUnlock(t testing.TB, req swarm.UnlockRequest) error {
	cli := d.NewClientT(t)
	defer cli.Close()

	_, err := cli.SwarmUnlock(context.Background(), client.SwarmUnlockOptions{Key: req.UnlockKey})
	if err != nil {
		err = errors.Wrap(err, "unlocking swarm")
	}
	return err
}

// GetSwarm returns the current swarm object
func (d *Daemon) GetSwarm(t testing.TB) swarm.Swarm {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.SwarmInspect(t.Context(), client.SwarmInspectOptions{})
	assert.NilError(t, err)
	return result.Swarm
}

// UpdateSwarm updates the current swarm object with the specified spec constructors
func (d *Daemon) UpdateSwarm(t testing.TB, f ...SpecConstructor) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	sw := d.GetSwarm(t)
	for _, fn := range f {
		fn(&sw.Spec)
	}

	_, err := cli.SwarmUpdate(t.Context(), client.SwarmUpdateOptions{
		Version: sw.Version,
		Spec:    sw.Spec,
	})
	assert.NilError(t, err)
}

// RotateTokens update the swarm to rotate tokens
func (d *Daemon) RotateTokens(t testing.TB) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.SwarmInspect(t.Context(), client.SwarmInspectOptions{})
	assert.NilError(t, err)

	_, err = cli.SwarmUpdate(t.Context(), client.SwarmUpdateOptions{
		Version:            result.Swarm.Version,
		Spec:               result.Swarm.Spec,
		RotateManagerToken: true,
		RotateWorkerToken:  true,
	})
	assert.NilError(t, err)
}

// JoinTokens returns the current swarm join tokens
func (d *Daemon) JoinTokens(t testing.TB) swarm.JoinTokens {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.SwarmInspect(t.Context(), client.SwarmInspectOptions{})
	assert.NilError(t, err)
	return result.Swarm.JoinTokens
}

func (d *Daemon) startArgs() []string {
	if d.swarmWithIptables {
		return startArgsWithIptables
	}
	return startArgs
}
