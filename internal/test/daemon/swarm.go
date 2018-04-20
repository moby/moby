package daemon

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/internal/test"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/pkg/errors"
)

const (
	// DefaultSwarmPort is the default port use for swarm in the tests
	DefaultSwarmPort       = 2477
	defaultSwarmListenAddr = "0.0.0.0"
)

// StartAndSwarmInit starts the daemon (with busybox) and init the swarm
func (d *Daemon) StartAndSwarmInit(t testingT) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	// avoid networking conflicts
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
	d.StartWithBusybox(t, args...)

	d.SwarmInit(t, swarm.InitRequest{})
}

// StartAndSwarmJoin starts the daemon (with busybox) and join the specified swarm as worker or manager
func (d *Daemon) StartAndSwarmJoin(t testingT, leader *Daemon, manager bool) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	// avoid networking conflicts
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
	d.StartWithBusybox(t, args...)

	tokens := leader.JoinTokens(t)
	token := tokens.Worker
	if manager {
		token = tokens.Manager
	}
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
func (d *Daemon) SwarmInit(t assert.TestingT, req swarm.InitRequest) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	cli := d.NewClientT(t)
	defer cli.Close()
	_, err := cli.SwarmInit(context.Background(), req)
	assert.NilError(t, err, "initializing swarm")
	d.CachedInfo = d.Info(t)
}

// SwarmJoin joins a daemon to an existing cluster.
func (d *Daemon) SwarmJoin(t assert.TestingT, req swarm.JoinRequest) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	if req.ListenAddr == "" {
		req.ListenAddr = fmt.Sprintf("%s:%d", d.swarmListenAddr, d.SwarmPort)
	}
	cli := d.NewClientT(t)
	defer cli.Close()
	err := cli.SwarmJoin(context.Background(), req)
	assert.NilError(t, err, "initializing swarm")
	d.CachedInfo = d.Info(t)
}

// SwarmLeave forces daemon to leave current cluster.
func (d *Daemon) SwarmLeave(force bool) error {
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("leaving swarm: failed to create client %v", err)
	}
	defer cli.Close()
	err = cli.SwarmLeave(context.Background(), force)
	if err != nil {
		err = fmt.Errorf("leaving swarm: %v", err)
	}
	return err
}

// SwarmInfo returns the swarm information of the daemon
func (d *Daemon) SwarmInfo(t assert.TestingT) swarm.Info {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	info, err := cli.Info(context.Background())
	assert.NilError(t, err, "get swarm info")
	return info.Swarm
}

// SwarmUnlock tries to unlock a locked swarm
func (d *Daemon) SwarmUnlock(req swarm.UnlockRequest) error {
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("unlocking swarm: failed to create client %v", err)
	}
	defer cli.Close()
	err = cli.SwarmUnlock(context.Background(), req)
	if err != nil {
		err = errors.Wrap(err, "unlocking swarm")
	}
	return err
}

// GetSwarm returns the current swarm object
func (d *Daemon) GetSwarm(t assert.TestingT) swarm.Swarm {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(t, err)
	return sw
}

// UpdateSwarm updates the current swarm object with the specified spec constructors
func (d *Daemon) UpdateSwarm(t assert.TestingT, f ...SpecConstructor) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
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
func (d *Daemon) RotateTokens(t assert.TestingT) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
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
func (d *Daemon) JoinTokens(t assert.TestingT) swarm.JoinTokens {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	assert.NilError(t, err)
	return sw.JoinTokens
}
