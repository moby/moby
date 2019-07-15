package swarm // import "github.com/docker/docker/integration/swarm"

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/daemon"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

// TestSwarmLeaderElection tests the swarm leader election behavior. It creates
// three daemons in a swarm, all managers, and then covers three cases:
//   1. Adding nodes after the first one should not cause a leader election
//   2. Stopping the leader should cause a leader election
//   3. Adding the stopped node back should not cause a leader election
func TestSwarmLeaderElection(t *testing.T) {
	defer setupTest(t)()

	skip.If(t, runtime.GOARCH == "s390x", "disabled on s390x")
	skip.If(t, runtime.GOARCH == "ppc64le", "disabled on ppc64le")

	daemons := [3]*daemon.Daemon{}
	names := map[string]string{}

	defer func() {
		for _, d := range daemons {
			if d != nil {
				d.Stop(t)
			}
		}
	}()

	// this closure expresses the name of a daemon as "daemons[i]" instead of
	// the daemon ID, which makes the test logs easier to understand
	name := func(d *daemon.Daemon) string {
		return names[d.ID()]
	}

	// Create 3 daemons, all managers
	daemons[0] = swarm.NewSwarm(t, testEnv, daemon.WithExperimental)
	names[daemons[0].ID()] = "daemons[0]"
	t.Logf("initial manager (%v) has ID: %v", name(daemons[0]), daemons[0].ID())

	daemons[1] = daemon.New(t, daemon.WithExperimental, daemon.WithSwarmPort(daemon.DefaultSwarmPort+1))
	daemons[1].StartAndSwarmJoin(t, daemons[0], true)
	names[daemons[1].ID()] = "daemons[1]"
	t.Logf("first joined manager (%v) has ID: %v", name(daemons[1]), daemons[1].ID())

	daemons[2] = daemon.New(t, daemon.WithExperimental, daemon.WithSwarmPort(daemon.DefaultSwarmPort+2))
	daemons[2].StartAndSwarmJoin(t, daemons[0], true)
	names[daemons[2].ID()] = "daemons[2]"
	t.Logf("second joined manager (%v) has ID: %v", name(daemons[2]), daemons[2].ID())

	isLeader := func(d *daemon.Daemon, p poll.LogT) bool {
		p.Logf("checking if %v is the leader", name(d))

		// get the node directly, instead of through GetNode, so we
		// can time out early if the attempt fails
		cli := d.NewClientT(t)
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		node, _, err := cli.NodeInspectWithRaw(ctx, d.NodeID())
		if err != nil {
			p.Logf("%v did not respond to node inspect: %v", name(d), err)
			return false
		}

		return node.ManagerStatus.Leader
	}

	// find the first leader
	var leader *daemon.Daemon
	// these poll functions are all quite similar, but different enough that
	// they're not coalesced into one big function.
	poll.WaitOn(t, func(p poll.LogT) poll.Result {
		// we're looking, in this poll, for 1 leader and 2 followers.
		numLeaders := 0
		numFollowers := 0
		for _, d := range daemons {
			if isLeader(d, p) {
				leader = d
				numLeaders++
				p.Logf("%v is a leader", name(d))
			} else {
				numFollowers++
				p.Logf("%v is not a leader", name(d))
			}
		}
		if numLeaders == 0 {
			return poll.Continue("no leader yet found")
		}
		if numLeaders > 1 {
			// this case should not occur, but if it did, this would raise an
			// immediate red flag and should totally stop the test.
			return poll.Error(fmt.Errorf("more than one node claims to be the leader"))
		}
		if numFollowers != 2 {
			return poll.Continue("number of followers is wrong, we expect 2 but have %v", numFollowers)
		}
		return poll.Success()
	})
	t.Logf("first leader is %v", name(leader))
	assert.Check(
		t, is.Equal(leader, daemons[0]),
		"%v was the first node in the swarm, and adding nodes"+
			"should not have caused a leader election, but %v is the leader",
		name(daemons[0]), name(leader),
	)

	leader.Stop(t)
	t.Logf("stopped daemon %v", name(leader))

	// now, wait for another leader to be selected
	// for some reason, leadership election can take a long time, so uhhh
	//
	// i guess we'll let it take a long time.
	var newLeader *daemon.Daemon
	poll.WaitOn(t, func(p poll.LogT) poll.Result {
		// this time, we should only have 1 follower.
		numLeaders := 0
		numFollowers := 0
		// skip daemons[0], which is the old leader
		for _, d := range daemons[1:] {
			if isLeader(d, p) {
				newLeader = d
				numLeaders++
				p.Logf("%v is a leader", name(d))
			} else {
				numFollowers++
				p.Logf("%v is not a leader", name(d))
			}
		}
		if numLeaders == 0 {
			return poll.Continue("no leader yet found")
		}
		if numLeaders > 1 {
			// this case should not occur, but if it did, this would raise an
			// immediate red flag and should totally stop the test.
			return poll.Error(fmt.Errorf("more than one node claims to be the leader"))
		}
		if numFollowers != 1 {
			return poll.Continue("number of followers is wrong, we expect 1 but have %v", numFollowers)
		}
		return poll.Success()
	}, poll.WithTimeout(30*time.Second))

	t.Logf("leader after stopping the old leader is %v", name(newLeader))

	// now, we'll start the old leader back up. We should still only have 1
	// leader
	leader.Start(t)

	// the Start method claims the daemon will be ready to recieve requests,
	// but some odd failures suggest otherwise. Instead, let's verify directly
	// that we can get the node info of the old leader
	poll.WaitOn(t, func(_ poll.LogT) poll.Result {
		cli := leader.NewClientT(t)
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, _, err := cli.NodeInspectWithRaw(ctx, leader.NodeID())

		if err == nil {
			return poll.Success()
		}
		return poll.Continue("node inspect failed: %v", err)
	}, poll.WithTimeout(30*time.Second))

	// now, do another check, and make sure the leader has not changed
	poll.WaitOn(t, func(p poll.LogT) poll.Result {
		// finally, we should have 2 followers again, but the leader should be
		// the same as the last iteration.
		foundLeader := false
		numFollowers := 0
		for _, d := range daemons {
			if isLeader(d, p) {
				if d != newLeader {
					return poll.Error(fmt.Errorf(
						"adding back %v caused a leader election,"+
							"and now %v is the leader (not %v)",
						name(leader), name(d), name(newLeader),
					))
				}
				foundLeader = true
			} else {
				numFollowers++
			}
		}

		if !foundLeader {
			return poll.Continue("have not yet figured out the leader")
		}
		if numFollowers != 2 {
			return poll.Continue("number of followers is wrong, we expect 1 but have %v", numFollowers)
		}
		return poll.Success()
	}, poll.WithTimeout(20*time.Second))
}
