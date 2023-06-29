package daemon

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

const (
	// DefaultSwarmPort is the default port use for swarm in the tests
	DefaultSwarmPort       = 2477
	defaultSwarmListenAddr = "0.0.0.0"
)

var startArgs = []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}

// StartNode (re)starts the daemon
func (d *Daemon) StartNode(t testing.TB) {
	t.Helper()
	d.Start(t, startArgs...)
}

// StartNodeWithBusybox starts daemon to be used as a swarm node, and loads the busybox image
func (d *Daemon) StartNodeWithBusybox(t testing.TB) {
	t.Helper()
	d.StartWithBusybox(t, startArgs...)
}

// RestartNode restarts a daemon to be used as a swarm node
func (d *Daemon) RestartNode(t testing.TB) {
	t.Helper()
	// avoid networking conflicts
	d.Stop(t)
	d.Start(t, startArgs...)
}

// StartAndSwarmInit starts the daemon (with busybox) and init the swarm
func (d *Daemon) StartAndSwarmInit(t testing.TB) {
	d.StartNodeWithBusybox(t)
	d.SwarmInit(t, swarm.InitRequest{})
}

// StartAndSwarmJoin starts the daemon (with busybox) and join the specified swarm as worker or manager
func (d *Daemon) StartAndSwarmJoin(t testing.TB, leader *Daemon, manager bool) {
	t.Helper()
	d.StartNodeWithBusybox(t)

	tokens := leader.JoinTokens(t)
	token := tokens.Worker
	if manager {
		token = tokens.Manager
	}
	t.Logf("[%s] joining swarm manager [%s]@%s, swarm listen addr %s", d.id, leader.id, leader.SwarmListenAddr(), d.SwarmListenAddr())
	d.SwarmJoin(t, swarm.JoinRequest{
		RemoteAddrs: []string{leader.SwarmListenAddr()},
		JoinToken:   token,
	})
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

// SwarmInit initializes a new swarm cluster.
func (d *Daemon) SwarmInit(t testing.TB, req swarm.InitRequest) {
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
	_, err := cli.SwarmInit(context.Background(), req)
	assert.NilError(t, err, "initializing swarm")
	d.CachedInfo = d.Info(t)
}

// SwarmJoin joins a daemon to an existing cluster.
func (d *Daemon) SwarmJoin(t testing.TB, req swarm.JoinRequest) {
	t.Helper()
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	cli := d.NewClientT(t)
	defer cli.Close()
	err := cli.SwarmJoin(context.Background(), req)
	assert.NilError(t, err, "[%s] joining swarm", d.id)
	d.CachedInfo = d.Info(t)
}

// SwarmLeave forces daemon to leave current cluster.
//
// The passed in testing.TB is only used to validate that the client was successfully created
// Some tests rely on error checking the result of the actual unlock, so allow
// the error to be returned.
func (d *Daemon) SwarmLeave(t testing.TB, force bool) error {
	cli := d.NewClientT(t)
	defer cli.Close()
	return cli.SwarmLeave(context.Background(), force)
}

// SwarmInfo returns the swarm information of the daemon
func (d *Daemon) SwarmInfo(t testing.TB) swarm.Info {
	t.Helper()
	cli := d.NewClientT(t)
	info, err := cli.Info(context.Background())
	assert.NilError(t, err, "get swarm info")
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

	err := cli.SwarmUnlock(context.Background(), req)
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

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(t, err)
	return sw
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

	err := cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, swarm.UpdateFlags{})
	assert.NilError(t, err)
}

// RotateTokens update the swarm to rotate tokens
func (d *Daemon) RotateTokens(t testing.TB) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(t, err)

	flags := swarm.UpdateFlags{
		RotateManagerToken: true,
		RotateWorkerToken:  true,
	}

	err = cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, flags)
	assert.NilError(t, err)
}

// JoinTokens returns the current swarm join tokens
func (d *Daemon) JoinTokens(t testing.TB) swarm.JoinTokens {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(t, err)
	return sw.JoinTokens
}
