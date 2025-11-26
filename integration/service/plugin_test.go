package service

import (
	"bytes"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/mount"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fixtures/plugin"
	"github.com/moby/moby/v2/internal/testutil/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestServicePlugin(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.NotAmd64)
	ctx := setupTest(t)

	reg := registry.NewV2(t)
	defer reg.Close()

	name := "test-" + strings.ToLower(t.Name())
	repo := path.Join(registry.DefaultURL, "swarm", name+":v1")
	repo2 := path.Join(registry.DefaultURL, "swarm", name+":v2")

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	apiclient := d.NewClientT(t)
	err := plugin.Create(ctx, apiclient, repo)
	assert.NilError(t, err)
	r, err := apiclient.PluginPush(ctx, repo, client.PluginPushOptions{})
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, r)
	assert.NilError(t, err)
	_, err = apiclient.PluginRemove(ctx, repo, client.PluginRemoveOptions{})
	assert.NilError(t, err)
	err = plugin.Create(ctx, apiclient, repo2)
	assert.NilError(t, err)
	r, err = apiclient.PluginPush(ctx, repo2, client.PluginPushOptions{})
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, r)
	assert.NilError(t, err)
	_, err = apiclient.PluginRemove(ctx, repo2, client.PluginRemoveOptions{})
	assert.NilError(t, err)
	d.Stop(t)

	d1 := swarm.NewSwarm(ctx, t, testEnv, daemon.WithExperimental())
	defer d1.Stop(t)
	d2 := daemon.New(t, daemon.WithExperimental(), daemon.WithSwarmPort(daemon.DefaultSwarmPort+1))
	d2.StartAndSwarmJoin(ctx, t, d1, true)
	defer d2.Stop(t)
	d3 := daemon.New(t, daemon.WithExperimental(), daemon.WithSwarmPort(daemon.DefaultSwarmPort+2))
	d3.StartAndSwarmJoin(ctx, t, d1, false)
	defer d3.Stop(t)

	id := d1.CreateService(ctx, t, makePlugin(repo, name, nil))
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, name), swarm.ServicePoll)

	// test that environment variables are passed from plugin service to plugin instance
	service := d1.GetService(ctx, t, id)
	tasks := d1.GetServiceTasksWithFilters(ctx, t, service.Spec.Annotations.Name, make(client.Filters).Add("runtime", "plugin"))
	if len(tasks) == 0 {
		t.Log("No tasks found for plugin service")
		t.Fail()
	}
	res, err := d1.NewClientT(t).PluginInspect(ctx, name, client.PluginInspectOptions{})
	assert.NilError(t, err, "Error inspecting service plugin")
	found := false
	for _, env := range res.Plugin.Settings.Env {
		assert.Equal(t, strings.HasPrefix(env, "baz"), false, "Environment variable entry %q is invalid and should not be present", "baz")
		if strings.HasPrefix(env, "foo=") {
			found = true
			assert.Equal(t, env, "foo=bar")
		}
	}
	assert.Equal(t, true, found, "Environment variable %q not found in plugin", "foo")

	d1.UpdateService(ctx, t, service, makePlugin(repo2, name, nil))
	poll.WaitOn(t, d1.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginReferenceIs(t, name, repo2), swarm.ServicePoll)
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, name), swarm.ServicePoll)

	d1.RemoveService(ctx, t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	// constrain to managers only
	id = d1.CreateService(ctx, t, makePlugin(repo, name, []string{"node.role==manager"}))
	poll.WaitOn(t, d1.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	d1.RemoveService(ctx, t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, name), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, name), swarm.ServicePoll)

	// with no name
	id = d1.CreateService(ctx, t, makePlugin(repo, "", nil))
	poll.WaitOn(t, d1.PluginIsRunning(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsRunning(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsRunning(t, repo), swarm.ServicePoll)

	d1.RemoveService(ctx, t, id)
	poll.WaitOn(t, d1.PluginIsNotPresent(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d2.PluginIsNotPresent(t, repo), swarm.ServicePoll)
	poll.WaitOn(t, d3.PluginIsNotPresent(t, repo), swarm.ServicePoll)
}

func TestServiceCSIPlugin(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.NotAmd64)
	ctx := setupTest(t)

	reg := registry.NewV2(t)
	defer reg.Close()

	name := "test-" + strings.ToLower(t.Name())
	repo := path.Join(registry.DefaultURL, "swarm", name+":v1")
	err := plugin.CreateInRegistry(ctx, repo, nil, plugin.WithCSI())
	assert.NilError(t, err)

	d1 := swarm.NewSwarm(ctx, t, testEnv)
	defer d1.Stop(t)

	apiclient := d1.NewClientT(t)
	rdr, err := apiclient.PluginInstall(ctx, name, client.PluginInstallOptions{Disabled: false, RemoteRef: repo})
	assert.NilError(t, err)
	defer rdr.Close()

	_, err = io.Copy(io.Discard, rdr)
	assert.NilError(t, err)

	p, _, err := apiclient.PluginInspectWithRaw(ctx, name)
	assert.NilError(t, err)
	assert.Assert(t, p.Enabled)

	vName, err := d1.CreateVolume(ctx, t, makeVolume(name, name))
	assert.NilError(t, err)
	assert.Equal(t, vName, name)

	poll.WaitOn(t, d1.PollCheckLogs(ctx, daemon.ScanLogsMatchString("using cluster volume")), swarm.ServicePoll)
	poll.WaitOn(t, d1.PollCheckLogs(ctx, daemon.ScanLogsMatchString("updated volume")), swarm.ServicePoll)

	v := d1.GetVolume(ctx, t, name)
	assert.Assert(t, v.ClusterVolume.Info != nil)
	assert.Equal(t, v.ClusterVolume.Info.VolumeID, name)

	serviceID := swarm.CreateService(ctx, t, d1,
		swarm.ServiceWithMounts([]mount.Mount{
			{
				Type:   mount.TypeCluster,
				Source: name,
				Target: "/data",
			},
		}),
	)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiclient, serviceID, 1), swarm.ServicePoll)

	tasks := swarm.GetRunningTasks(ctx, t, apiclient, serviceID)
	assert.Assert(t, len(tasks) > 0, "no running tasks found for service %s", serviceID)

	resp := swarm.ExecTask(ctx, t, d1, tasks[0], client.ExecCreateOptions{
		Cmd:          []string{"/bin/cat", "/data/data.txt"},
		AttachStdout: true,
		AttachStderr: true,
	})
	var outBuf, errBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(outBuf.String(), "this is a csi mounted file"))
	assert.Check(t, is.Equal(errBuf.String(), ""))
}

func makePlugin(repo, name string, constraints []string) func(*swarmtypes.Service) {
	return func(s *swarmtypes.Service) {
		s.Spec.TaskTemplate.Runtime = swarmtypes.RuntimePlugin
		s.Spec.TaskTemplate.PluginSpec = &swarmtypes.RuntimeSpec{
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

func makeVolume(driver, name string) func(*volume.CreateOptions) {
	return func(v *volume.CreateOptions) {
		v.Driver = driver
		v.Name = name
		v.ClusterVolumeSpec = &volume.ClusterVolumeSpec{
			AccessMode: &volume.AccessMode{
				MountVolume: &volume.TypeMount{},
				Scope:       volume.ScopeSingleNode,
				Sharing:     volume.SharingNone,
			},
			AccessibilityRequirements: &volume.TopologyRequirement{},
			Availability:              volume.AvailabilityActive,
			CapacityRange: &volume.CapacityRange{
				LimitBytes:    0,
				RequiredBytes: 0,
			},
		}
	}
}
