package service

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/api/types"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/swarm/runtime"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/docker/docker/internal/test/fixtures/plugin"
	"github.com/docker/docker/internal/test/registry"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestServicePlugin(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTest(t)()

	reg := registry.NewV2(t)
	defer reg.Close()

	repo := path.Join(registry.DefaultURL, "swarm", "test:v1")
	repo2 := path.Join(registry.DefaultURL, "swarm", "test:v2")
	name := "test"

	d := daemon.New(t)
	d.StartWithBusybox(t)
	apiclient := d.NewClientT(t)
	err := plugin.Create(context.Background(), apiclient, repo)
	assert.NilError(t, err)
	r, err := apiclient.PluginPush(context.Background(), repo, "")
	assert.NilError(t, err)
	_, err = io.Copy(ioutil.Discard, r)
	assert.NilError(t, err)
	err = apiclient.PluginRemove(context.Background(), repo, types.PluginRemoveOptions{})
	assert.NilError(t, err)
	err = plugin.Create(context.Background(), apiclient, repo2)
	assert.NilError(t, err)
	r, err = apiclient.PluginPush(context.Background(), repo2, "")
	assert.NilError(t, err)
	_, err = io.Copy(ioutil.Discard, r)
	assert.NilError(t, err)
	err = apiclient.PluginRemove(context.Background(), repo2, types.PluginRemoveOptions{})
	assert.NilError(t, err)
	d.Stop(t)

	d1 := swarm.NewSwarm(t, testEnv, daemon.WithExperimental)
	defer d1.Stop(t)
	d2 := daemon.New(t, daemon.WithExperimental, daemon.WithSwarmPort(daemon.DefaultSwarmPort+1))
	d2.StartAndSwarmJoin(t, d1, true)
	defer d2.Stop(t)
	d3 := daemon.New(t, daemon.WithExperimental, daemon.WithSwarmPort(daemon.DefaultSwarmPort+2))
	d3.StartAndSwarmJoin(t, d1, false)
	defer d3.Stop(t)

	id := d1.CreateService(t, makePlugin(repo, name, nil))
	poll.WaitOn(t, d1.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(name), swarm.ServicePoll)

	service := d1.GetService(t, id)
	d1.UpdateService(t, service, makePlugin(repo2, name, nil))
	poll.WaitOn(t, d1.PluginReferenceIs(name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginReferenceIs(name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginReferenceIs(name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d1.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(name), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(name), swarm.ServicePoll)

	// constrain to managers only
	id = d1.CreateService(t, makePlugin(repo, name, []string{"node.role==manager"}))
	poll.WaitOn(t, d1.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(name), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(name), swarm.ServicePoll)

	// with no name
	id = d1.CreateService(t, makePlugin(repo, "", nil))
	poll.WaitOn(t, d1.PluginIsRunning(repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(repo), swarm.ServicePoll)

	d1.RemoveService(t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(repo), swarm.ServicePoll)
}

func makePlugin(repo, name string, constraints []string) func(*swarmtypes.Service) {
	return func(s *swarmtypes.Service) {
		s.Spec.TaskTemplate.Runtime = "plugin"
		s.Spec.TaskTemplate.PluginSpec = &runtime.PluginSpec{
			Name:   name,
			Remote: repo,
		}
		if constraints != nil {
			s.Spec.TaskTemplate.Placement = &swarmtypes.Placement{
				Constraints: constraints,
			}
		}
	}
}
