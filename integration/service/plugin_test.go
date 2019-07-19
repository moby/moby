package service

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/swarm/runtime"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/certutil"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/docker/docker/internal/test/fixtures/plugin"
	"github.com/docker/docker/internal/test/registry"
	"gotest.tools/assert"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestServicePlugin(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, os.Getenv("DOCKER_ENGINE_GOARCH") != "amd64")
	defer setupTest(t)()

	// TODO: We shouldn't really need to do this on a separate daemon, it's just
	// convenient for getting an IP address to use for the registry which is
	// accessible to all daemons...
	d, cleanup := daemon.New(t)
	defer cleanup(t)
	d.Start(t)

	certs, cleanupCerts := certutil.New(t, d.IP(t).String())
	defer cleanupCerts(t)

	reg := registry.NewV2(t, registry.URL(d.IP(t).String()+":5000"), registry.Exec(d.Exec), registry.TLS(certs))
	defer reg.Close()

	certsDir := "/etc/docker/certs.d/" + reg.URL()
	assert.NilError(t, os.MkdirAll(certsDir, 0700))
	defer os.RemoveAll(certsDir)
	caCert, err := ioutil.ReadFile(certs.CACertPath)
	assert.NilError(t, err)
	err = ioutil.WriteFile(filepath.Join(certsDir, "ca.crt"), caCert, 0600)
	assert.NilError(t, err)

	reg.WaitReady(t)

	repoPath1 := "swarm/test:v1"
	repoPath2 := "swarm/test:v2"

	repo := path.Join(reg.URL(), repoPath1)
	repo2 := path.Join(reg.URL(), repoPath2)
	name := "test"

	apiclient := d.NewClientT(t)

	err = plugin.Create(context.Background(), apiclient, repo)
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

	d1, cleanup1 := swarm.NewSwarm(t, testEnv, daemon.WithExperimental)
	defer cleanup1(t)
	defer d1.Stop(t)

	d2, cleanup2 := daemon.New(t, daemon.WithExperimental)
	defer cleanup2(t)
	d2.StartAndSwarmJoin(t, d1, true)
	defer d2.Stop(t)

	d3, cleanup3 := daemon.New(t, daemon.WithExperimental)
	defer cleanup3(t)
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
