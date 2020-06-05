package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

type serviceOpts struct {
	name         string
	network      string
	cmd          []string
	replicas     uint64
	healthConfig *container.HealthConfig
	mounts       []mount.Mount
}

func newServiceOpts() *serviceOpts {
	return &serviceOpts{
		name:     "test_node_lb_cleanup",
		network:  getTestNetName(),
		replicas: uint64(1),
	}
}

func getTestNetName() string {
	return "ol_test_net"
}

func pollSetting(config *poll.Settings) {
	config.Timeout = 100 * time.Second
	config.Delay = 100 * time.Millisecond
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		config.Timeout = 500 * time.Second
	}
}

func createService(t *testing.T, d *daemon.Daemon, o *serviceOpts) (serviceID string) {
	opts := []swarm.ServiceSpecOpt{
		swarm.ServiceWithReplicas(o.replicas),
		swarm.ServiceWithName(o.name),
		swarm.ServiceWithNetwork(o.network),
		swarm.ServiceWithCommand(o.cmd),
	}
	if o.healthConfig != nil {
		opts = append(opts, swarm.ServiceWithHealthCheck(o.healthConfig))
	}
	if o.mounts != nil {
		opts = append(opts, swarm.ServiceWithMount(o.mounts))
	}

	attempts := new(uint64)
	*attempts = uint64(1)
	delay := new(time.Duration)
	*delay = time.Second
	restartPolicy := &swarmtypes.RestartPolicy{
		MaxAttempts: attempts,
		Delay:       delay,
	}
	opts = append(opts, swarm.ServiceWithRestartPolicy(restartPolicy))

	serviceID = swarm.CreateService(t, d, opts...)
	return
}

func checkNetworkRemoved(ctx context.Context, t *testing.T, c *client.Client) error {
	// network removal is asynchronous, retry testing at most 30 secs.
	for count := 0; count < 10; count++ {
		time.Sleep(3 * time.Second)
		nr, err := c.NetworkInspect(ctx, getTestNetName(), types.NetworkInspectOptions{})
		assert.NilError(t, err)
		if nr.Containers == nil {
			return nil
		}
	}
	return errors.New("assertNetworkRemoved failed")
}

func testNormalService(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"sleep", "3d"}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.RunningTasksCount(c, serviceID, uint64(1)), pollSetting)
	err := c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
	err = checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
}

func testServiceStartFail(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"non-existing-file"}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.FailedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func testServiceCompletion(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"true"}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.CompletedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func testServiceExitFail(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"false"}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.FailedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func testServiceHealthCheckFail(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"sleep", "3d"}
	o.healthConfig = &container.HealthConfig{
		Test:        []string{"CMD-SHELL", "false"},
		Interval:    time.Second,
		Timeout:     time.Second,
		StartPeriod: time.Second,
		Retries:     1,
	}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.FailedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func testServiceHealthCheckExitEarly(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"sleep", "1s"}
	o.healthConfig = &container.HealthConfig{
		Test:        []string{"CMD-SHELL", "false"},
		Interval:    time.Second,
		Timeout:     time.Second,
		StartPeriod: time.Second,
		Retries:     3,
	}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.CompletedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func testServiceInvalidMount(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client) {
	o := newServiceOpts()
	o.cmd = []string{"sleep", "3d"}
	o.mounts = []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: "/non-existing",
			Target: "/ttt",
		},
	}
	serviceID := createService(t, d, o)
	poll.WaitOn(t, swarm.RejectedTasksCount(c, serviceID, uint64(2)), pollSetting)
	err := checkNetworkRemoved(ctx, t, c)
	assert.NilError(t, err)
	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func TestNodeLBCleanup(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	ctx := context.Background()

	// create an overlay network
	net := getTestNetName()
	netID := network.CreateNoError(ctx, t, c, net,
		network.WithDriver("overlay"))

	t.Run("testNormalService", func(t *testing.T) { testNormalService(ctx, t, d, c) })
	t.Run("testServiceStartFail", func(t *testing.T) { testServiceStartFail(ctx, t, d, c) })
	t.Run("testServiceCompletion", func(t *testing.T) { testServiceCompletion(ctx, t, d, c) })
	t.Run("testServiceExitFail", func(t *testing.T) { testServiceExitFail(ctx, t, d, c) })
	t.Run("testServiceHealthCheckFail", func(t *testing.T) { testServiceHealthCheckFail(ctx, t, d, c) })
	t.Run("testServiceHealthCheckExitEarly", func(t *testing.T) { testServiceHealthCheckExitEarly(ctx, t, d, c) })
	t.Run("testServiceInvalidMount", func(t *testing.T) { testServiceInvalidMount(ctx, t, d, c) })

	err := c.NetworkRemove(ctx, netID)
	assert.NilError(t, err)
	err = d.SwarmLeave(t, true)
	assert.NilError(t, err)

}
