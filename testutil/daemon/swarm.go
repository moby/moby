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

var (
	startArgsWithIptables = []string{"--swarm-default-advertise-addr=lo"}
	startArgs             = []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
)

// StartNode (re)starts the daemon
func (d *Daemon) StartNode(tb testing.TB) {
	tb.Helper()
	d.Start(tb, d.startArgs()...)
}

// StartNodeWithBusybox starts daemon to be used as a swarm node, and loads the busybox image
func (d *Daemon) StartNodeWithBusybox(ctx context.Context, tb testing.TB) {
	tb.Helper()
	d.StartWithBusybox(ctx, tb, d.startArgs()...)
}

// RestartNode restarts a daemon to be used as a swarm node
func (d *Daemon) RestartNode(tb testing.TB) {
	tb.Helper()
	// avoid networking conflicts
	d.Stop(tb)
	d.Start(tb, d.startArgs()...)
}

// StartAndSwarmInit starts the daemon (with busybox) and init the swarm
func (d *Daemon) StartAndSwarmInit(ctx context.Context, tb testing.TB) {
	d.StartNodeWithBusybox(ctx, tb)
	d.SwarmInit(ctx, tb, swarm.InitRequest{})
}

// StartAndSwarmJoin starts the daemon (with busybox) and join the specified swarm as worker or manager
func (d *Daemon) StartAndSwarmJoin(ctx context.Context, tb testing.TB, leader *Daemon, manager bool) {
	tb.Helper()
	d.StartNodeWithBusybox(ctx, tb)

	tokens := leader.JoinTokens(tb)
	token := tokens.Worker
	if manager {
		token = tokens.Manager
	}
	tb.Logf("[%s] joining swarm manager [%s]@%s, swarm listen addr %s", d.id, leader.id, leader.SwarmListenAddr(), d.SwarmListenAddr())
	d.SwarmJoin(ctx, tb, swarm.JoinRequest{
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
func (d *Daemon) SwarmInit(ctx context.Context, tb testing.TB, req swarm.InitRequest) {
	tb.Helper()
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
	cli := d.NewClientT(tb)
	defer cli.Close()
	_, err := cli.SwarmInit(ctx, req)
	assert.NilError(tb, err, "initializing swarm")
	d.CachedInfo = d.Info(tb)
}

// SwarmJoin joins a daemon to an existing cluster.
func (d *Daemon) SwarmJoin(ctx context.Context, tb testing.TB, req swarm.JoinRequest) {
	tb.Helper()
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	cli := d.NewClientT(tb)
	defer cli.Close()
	err := cli.SwarmJoin(ctx, req)
	assert.NilError(tb, err, "[%s] joining swarm", d.id)
	d.CachedInfo = d.Info(tb)
}

// SwarmLeave forces daemon to leave current cluster.
//
// The passed in testing.TB is only used to validate that the client was successfully created
// Some tests rely on error checking the result of the actual unlock, so allow
// the error to be returned.
func (d *Daemon) SwarmLeave(ctx context.Context, tb testing.TB, force bool) error {
	cli := d.NewClientT(tb)
	defer cli.Close()
	return cli.SwarmLeave(ctx, force)
}

// SwarmInfo returns the swarm information of the daemon
func (d *Daemon) SwarmInfo(ctx context.Context, tb testing.TB) swarm.Info {
	tb.Helper()
	cli := d.NewClientT(tb)
	info, err := cli.Info(ctx)
	assert.NilError(tb, err, "get swarm info")
	return info.Swarm
}

// SwarmUnlock tries to unlock a locked swarm
//
// The passed in testing.TB is only used to validate that the client was successfully created
// Some tests rely on error checking the result of the actual unlock, so allow
// the error to be returned.
func (d *Daemon) SwarmUnlock(tb testing.TB, req swarm.UnlockRequest) error {
	cli := d.NewClientT(tb)
	defer cli.Close()

	err := cli.SwarmUnlock(context.Background(), req)
	if err != nil {
		err = errors.Wrap(err, "unlocking swarm")
	}
	return err
}

// GetSwarm returns the current swarm object
func (d *Daemon) GetSwarm(tb testing.TB) swarm.Swarm {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(tb, err)
	return sw
}

// UpdateSwarm updates the current swarm object with the specified spec constructors
func (d *Daemon) UpdateSwarm(tb testing.TB, f ...SpecConstructor) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	sw := d.GetSwarm(tb)
	for _, fn := range f {
		fn(&sw.Spec)
	}

	err := cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, swarm.UpdateFlags{})
	assert.NilError(tb, err)
}

// RotateTokens update the swarm to rotate tokens
func (d *Daemon) RotateTokens(tb testing.TB) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(tb, err)

	flags := swarm.UpdateFlags{
		RotateManagerToken: true,
		RotateWorkerToken:  true,
	}

	err = cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, flags)
	assert.NilError(tb, err)
}

// JoinTokens returns the current swarm join tokens
func (d *Daemon) JoinTokens(tb testing.TB) swarm.JoinTokens {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(tb, err)
	return sw.JoinTokens
}

func (d *Daemon) startArgs() []string {
	if d.swarmWithIptables {
		return startArgsWithIptables
	}
	return startArgs
}
