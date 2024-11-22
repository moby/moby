package service // import "github.com/docker/docker/integration/service"

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/strslice"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestServiceCreateInit(t *testing.T) {
	ctx := setupTest(t)
	t.Run("daemonInitDisabled", testServiceCreateInit(ctx, false))
	t.Run("daemonInitEnabled", testServiceCreateInit(ctx, true))
}

func testServiceCreateInit(ctx context.Context, daemonEnabled bool) func(t *testing.T) {
	return func(t *testing.T) {
		_ = testutil.StartSpan(ctx, t)
		ops := []daemon.Option{}

		if daemonEnabled {
			ops = append(ops, daemon.WithInit())
		}
		d := swarm.NewSwarm(ctx, t, testEnv, ops...)
		defer d.Stop(t)
		client := d.NewClientT(t)
		defer client.Close()

		booleanTrue := true
		booleanFalse := false

		serviceID := swarm.CreateService(ctx, t, d)
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, 1), swarm.ServicePoll)
		i := inspectServiceContainer(ctx, t, client, serviceID)
		// HostConfig.Init == nil means that it delegates to daemon configuration
		assert.Check(t, is.Nil(i.HostConfig.Init))

		serviceID = swarm.CreateService(ctx, t, d, swarm.ServiceWithInit(&booleanTrue))
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, 1), swarm.ServicePoll)
		i = inspectServiceContainer(ctx, t, client, serviceID)
		assert.Check(t, is.Equal(true, *i.HostConfig.Init))

		serviceID = swarm.CreateService(ctx, t, d, swarm.ServiceWithInit(&booleanFalse))
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, 1), swarm.ServicePoll)
		i = inspectServiceContainer(ctx, t, client, serviceID)
		assert.Check(t, is.Equal(false, *i.HostConfig.Init))
	}
}

func inspectServiceContainer(ctx context.Context, t *testing.T, client client.APIClient, serviceID string) container.InspectResponse {
	t.Helper()
	containers, err := client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.swarm.service.id="+serviceID)),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(containers, 1))

	i, err := client.ContainerInspect(ctx, containers[0].ID)
	assert.NilError(t, err)
	return i
}

func TestCreateServiceMultipleTimes(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	overlayName := "overlay1_" + t.Name()
	overlayID := network.CreateNoError(ctx, t, client, overlayName,
		network.WithDriver("overlay"),
	)

	var instances uint64 = 4

	serviceName := "TestService_" + t.Name()
	serviceSpec := []swarm.ServiceSpecOpt{
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(overlayName),
	}

	serviceID := swarm.CreateService(ctx, t, d, serviceSpec...)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, instances), swarm.ServicePoll)

	_, _, err := client.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	err = client.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)

	poll.WaitOn(t, swarm.NoTasksForService(ctx, client, serviceID), swarm.ServicePoll)

	serviceID2 := swarm.CreateService(ctx, t, d, serviceSpec...)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID2, instances), swarm.ServicePoll)

	err = client.ServiceRemove(ctx, serviceID2)
	assert.NilError(t, err)

	// we can't just wait on no tasks for the service, counter-intuitively.
	// Tasks may briefly exist but not show up, if they are in the process
	// of being deallocated. To avoid this case, we should retry network remove
	// a few times, to give tasks time to be deallocated
	poll.WaitOn(t, swarm.NoTasksForService(ctx, client, serviceID2), swarm.ServicePoll)

	for retry := 0; retry < 5; retry++ {
		err = client.NetworkRemove(ctx, overlayID)
		// TODO(dperny): using strings.Contains for error checking is awful,
		// but so is the fact that swarm functions don't return errdefs errors.
		// I don't have time at this moment to fix the latter, so I guess I'll
		// go with the former.
		//
		// The full error we're looking for is something like this:
		//
		// Error response from daemon: rpc error: code = FailedPrecondition desc = network %v is in use by task %v
		//
		// The safest way to catch this, I think, will be to match on "is in
		// use by", as this is an uninterrupted string that best identifies
		// this error.
		if err == nil || !strings.Contains(err.Error(), "is in use by") {
			// if there is no error, or the error isn't this kind of error,
			// then we'll break the loop body, and either fail the test or
			// continue.
			break
		}
	}
	assert.NilError(t, err)

	poll.WaitOn(t, network.IsRemoved(ctx, client, overlayID), poll.WithTimeout(1*time.Minute), poll.WithDelay(10*time.Second))
}

func TestCreateServiceConflict(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	serviceName := "TestService_" + t.Name()
	serviceSpec := []swarm.ServiceSpecOpt{
		swarm.ServiceWithName(serviceName),
	}

	swarm.CreateService(ctx, t, d, serviceSpec...)

	spec := swarm.CreateServiceSpec(t, serviceSpec...)
	_, err := c.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	assert.Check(t, errdefs.IsConflict(err))
	assert.ErrorContains(t, err, "service "+serviceName+" already exists")
}

func TestCreateServiceMaxReplicas(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	var maxReplicas uint64 = 2
	serviceSpec := []swarm.ServiceSpecOpt{
		swarm.ServiceWithReplicas(maxReplicas),
		swarm.ServiceWithMaxReplicas(maxReplicas),
	}

	serviceID := swarm.CreateService(ctx, t, d, serviceSpec...)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, maxReplicas), swarm.ServicePoll)

	_, _, err := client.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)
}

func TestCreateServiceSecretFileMode(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	secretName := "TestSecret_" + t.Name()
	secretResp, err := client.SecretCreate(ctx, swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: secretName,
		},
		Data: []byte("TESTSECRET"),
	})
	assert.NilError(t, err)

	var instances uint64 = 1
	serviceName := "TestService_" + t.Name()
	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithCommand([]string{"/bin/sh", "-c", "ls -l /etc/secret && sleep inf"}),
		swarm.ServiceWithSecret(&swarmtypes.SecretReference{
			File: &swarmtypes.SecretReferenceFileTarget{
				Name: "/etc/secret",
				UID:  "0",
				GID:  "0",
				Mode: 0o777,
			},
			SecretID:   secretResp.ID,
			SecretName: secretName,
		}),
	)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, instances), swarm.ServicePoll)

	body, err := client.ServiceLogs(ctx, serviceID, container.LogsOptions{
		Tail:       "1",
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer body.Close()

	content, err := io.ReadAll(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(content), "-rwxrwxrwx"))

	err = client.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
	poll.WaitOn(t, swarm.NoTasksForService(ctx, client, serviceID), swarm.ServicePoll)

	err = client.SecretRemove(ctx, secretName)
	assert.NilError(t, err)
}

func TestCreateServiceConfigFileMode(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	configName := "TestConfig_" + t.Name()
	configResp, err := client.ConfigCreate(ctx, swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: configName,
		},
		Data: []byte("TESTCONFIG"),
	})
	assert.NilError(t, err)

	var instances uint64 = 1
	serviceName := "TestService_" + t.Name()
	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithCommand([]string{"/bin/sh", "-c", "ls -l /etc/config && sleep inf"}),
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithConfig(&swarmtypes.ConfigReference{
			File: &swarmtypes.ConfigReferenceFileTarget{
				Name: "/etc/config",
				UID:  "0",
				GID:  "0",
				Mode: 0o777,
			},
			ConfigID:   configResp.ID,
			ConfigName: configName,
		}),
	)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, instances))

	body, err := client.ServiceLogs(ctx, serviceID, container.LogsOptions{
		Tail:       "1",
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer body.Close()

	content, err := io.ReadAll(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(content), "-rwxrwxrwx"))

	err = client.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
	poll.WaitOn(t, swarm.NoTasksForService(ctx, client, serviceID))

	err = client.ConfigRemove(ctx, configName)
	assert.NilError(t, err)
}

// TestCreateServiceSysctls tests that a service created with sysctl options in
// the ContainerSpec correctly applies those options.
//
// To test this, we're going to create a service with the sysctl option
//
//	{"net.ipv4.ip_nonlocal_bind": "0"}
//
// We'll get the service's tasks to get the container ID, and then we'll
// inspect the container. If the output of the container inspect contains the
// sysctl option with the correct value, we can assume that the sysctl has been
// plumbed correctly.
//
// Next, we'll remove that service and create a new service with that option
// set to 1. This means that no matter what the default is, we can be confident
// that the sysctl option is applying as intended.
//
// Additionally, we'll do service and task inspects to verify that the inspect
// output includes the desired sysctl option.
//
// We're using net.ipv4.ip_nonlocal_bind because it's something that I'm fairly
// confident won't be modified by the container runtime, and won't blow
// anything up in the test environment
func TestCreateServiceSysctls(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	// run this block twice, so that no matter what the default value of
	// net.ipv4.ip_nonlocal_bind is, we can verify that setting the sysctl
	// options works
	for _, expected := range []string{"0", "1"} {
		// store the map we're going to be using everywhere.
		expectedSysctls := map[string]string{"net.ipv4.ip_nonlocal_bind": expected}

		// Create the service with the sysctl options
		var instances uint64 = 1
		serviceID := swarm.CreateService(ctx, t, d,
			swarm.ServiceWithSysctls(expectedSysctls),
		)

		// wait for the service to converge to 1 running task as expected
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, instances))

		// we're going to check 3 things:
		//
		//   1. Does the container, when inspected, have the sysctl option set?
		//   2. Does the task have the sysctl in the spec?
		//   3. Does the service have the sysctl in the spec?
		//
		// if all 3 of these things are true, we know that the sysctl has been
		// plumbed correctly through the engine.
		//
		// We don't actually have to get inside the container and check its
		// logs or anything. If we see the sysctl set on the container inspect,
		// we know that the sysctl is plumbed correctly. everything below that
		// level has been tested elsewhere. (thanks @thaJeztah, because an
		// earlier version of this test had to get container logs and was much
		// more complex)

		// get all tasks of the service, so we can get the container
		tasks, err := client.TaskList(ctx, types.TaskListOptions{
			Filters: filters.NewArgs(filters.Arg("service", serviceID)),
		})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(len(tasks), 1))

		// verify that the container has the sysctl option set
		ctnr, err := client.ContainerInspect(ctx, tasks[0].Status.ContainerStatus.ContainerID)
		assert.NilError(t, err)
		assert.DeepEqual(t, ctnr.HostConfig.Sysctls, expectedSysctls)

		// verify that the task has the sysctl option set in the task object
		assert.DeepEqual(t, tasks[0].Spec.ContainerSpec.Sysctls, expectedSysctls)

		// verify that the service also has the sysctl set in the spec.
		service, _, err := client.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
		assert.NilError(t, err)
		assert.DeepEqual(t,
			service.Spec.TaskTemplate.ContainerSpec.Sysctls, expectedSysctls,
		)
	}
}

// TestCreateServiceCapabilities tests that a service created with capabilities options in
// the ContainerSpec correctly applies those options.
//
// To test this, we're going to create a service with the capabilities option
//
//	[]string{"CAP_NET_RAW", "CAP_SYS_CHROOT"}
//
// We'll get the service's tasks to get the container ID, and then we'll
// inspect the container. If the output of the container inspect contains the
// capabilities option with the correct value, we can assume that the capabilities has been
// plumbed correctly.
func TestCreateServiceCapabilities(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	// store the map we're going to be using everywhere.
	capAdd := []string{"CAP_SYS_CHROOT"}
	capDrop := []string{"CAP_NET_RAW"}

	// Create the service with the capabilities options
	var instances uint64 = 1
	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithCapabilities(capAdd, capDrop),
	)

	// wait for the service to converge to 1 running task as expected
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, client, serviceID, instances))

	// we're going to check 3 things:
	//
	//   1. Does the container, when inspected, have the capabilities option set?
	//   2. Does the task have the capabilities in the spec?
	//   3. Does the service have the capabilities in the spec?
	//
	// if all 3 of these things are true, we know that the capabilities has been
	// plumbed correctly through the engine.
	//
	// We don't actually have to get inside the container and check its
	// logs or anything. If we see the capabilities set on the container inspect,
	// we know that the capabilities is plumbed correctly. everything below that
	// level has been tested elsewhere.

	// get all tasks of the service, so we can get the container
	tasks, err := client.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(filters.Arg("service", serviceID)),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(tasks), 1))

	// verify that the container has the capabilities option set
	ctnr, err := client.ContainerInspect(ctx, tasks[0].Status.ContainerStatus.ContainerID)
	assert.NilError(t, err)
	assert.DeepEqual(t, ctnr.HostConfig.CapAdd, strslice.StrSlice(capAdd))
	assert.DeepEqual(t, ctnr.HostConfig.CapDrop, strslice.StrSlice(capDrop))

	// verify that the task has the capabilities option set in the task object
	assert.DeepEqual(t, tasks[0].Spec.ContainerSpec.CapabilityAdd, capAdd)
	assert.DeepEqual(t, tasks[0].Spec.ContainerSpec.CapabilityDrop, capDrop)

	// verify that the service also has the capabilities set in the spec.
	service, _, err := client.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, service.Spec.TaskTemplate.ContainerSpec.CapabilityAdd, capAdd)
	assert.DeepEqual(t, service.Spec.TaskTemplate.ContainerSpec.CapabilityDrop, capDrop)
}
