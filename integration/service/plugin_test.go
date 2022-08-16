package service

import (
	"context"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/swarm/runtime"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/docker/docker/testutil/registry"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestServicePlugin(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTest(t)()

	reg := registry.NewV2(t)
	defer reg.Close()

	name := "test-" + strings.ToLower(t.Name())
	repo := path.Join(registry.DefaultURL, "swarm", name+":v1")
	repo2 := path.Join(registry.DefaultURL, "swarm", name+":v2")

	d := daemon.New(t)
	d.StartWithBusybox(t)
	apiclient := d.NewClientT(t)
	err := plugin.Create(context.Background(), apiclient, repo)
	assert.NilError(t, err)
	r, err := apiclient.PluginPush(context.Background(), repo, "")
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, r)
	assert.NilError(t, err)
	err = apiclient.PluginRemove(context.Background(), repo, types.PluginRemoveOptions{})
	assert.NilError(t, err)
	err = plugin.Create(context.Background(), apiclient, repo2)
	assert.NilError(t, err)
	r, err = apiclient.PluginPush(context.Background(), repo2, "")
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, r)
	assert.NilError(t, err)
	err = apiclient.PluginRemove(context.Background(), repo2, types.PluginRemoveOptions{})
	assert.NilError(t, err)
	d.Stop(t)

	d1 := swarm.NewSwarm(t, testEnv, daemon.WithExperimental())
	defer d1.Stop(t)
	d2 := daemon.New(t, daemon.WithExperimental(), daemon.WithSwarmPort(daemon.DefaultSwarmPort+1))
	d2.StartAndSwarmJoin(t, d1, true)
	defer d2.Stop(t)
	d3 := daemon.New(t, daemon.WithExperimental(), daemon.WithSwarmPort(daemon.DefaultSwarmPort+2))
	d3.StartAndSwarmJoin(t, d1, false)
	defer d3.Stop(t)

	id := d1.CreateService(t, makePlugin(repo, name, nil))
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, name), swarm.ServicePoll)

	// test that environment variables are passed from plugin service to plugin instance
	service := d1.GetService(t, id)
	tasks := d1.GetServiceTasks(t, service.Spec.Annotations.Name, filters.Arg("runtime", "plugin"))
	if len(tasks) == 0 {
		t.Log("No tasks found for plugin service")
		t.Fail()
	}
	plugin, _, err := d1.NewClientT(t).PluginInspectWithRaw(context.Background(), name)
	assert.NilError(t, err, "Error inspecting service plugin")
	found := false
	for _, env := range plugin.Settings.Env {
		assert.Equal(t, strings.HasPrefix(env, "baz"), false, "Environment variable entry %q is invalid and should not be present", "baz")
		if strings.HasPrefix(env, "foo=") {
			found = true
			assert.Equal(t, env, "foo=bar")
		}
	}
	assert.Equal(t, true, found, "Environment variable %q not found in plugin", "foo")

	d1.UpdateService(t, service, makePlugin(repo2, name, nil))
	poll.WaitOn(t, d1.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, name), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	// constrain to managers only
	id = d1.CreateService(t, makePlugin(repo, name, []string{"node.role==manager"}))
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	// with no name
	id = d1.CreateService(t, makePlugin(repo, "", nil))
	poll.WaitOn(t, d1.PluginIsRunning(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, repo), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, repo), swarm.ServicePoll)
}

func makePlugin(repo, name string, constraints []string) func(*swarmtypes.Service) {
	return func(s *swarmtypes.Service) {
		s.Spec.TaskTemplate.Runtime = swarmtypes.RuntimePlugin
		s.Spec.TaskTemplate.PluginSpec = &runtime.PluginSpec{
			Name:   name,
			Remote: repo,
			Env: []string{
				"baz",     // invalid environment variable entries are ignored
				"foo=bar", // "foo" will be the single environment variable
			},
		}
		if constraints != nil {
			s.Spec.TaskTemplate.Placement = &swarmtypes.Placement{
				Constraints: constraints,
			}
		}
	}
}
