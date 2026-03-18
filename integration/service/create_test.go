package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/container"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
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
		apiClient := d.NewClientT(t)
		defer apiClient.Close()

		booleanTrue := true
		booleanFalse := false

		serviceID := swarm.CreateService(ctx, t, d)
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1), swarm.ServicePoll)
		i := inspectServiceContainer(ctx, t, apiClient, serviceID)
		// HostConfig.Init == nil means that it delegates to daemon configuration
		assert.Check(t, is.Nil(i.HostConfig.Init))

		serviceID = swarm.CreateService(ctx, t, d, swarm.ServiceWithInit(&booleanTrue))
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1), swarm.ServicePoll)
		i = inspectServiceContainer(ctx, t, apiClient, serviceID)
		assert.Check(t, is.Equal(true, *i.HostConfig.Init))

		serviceID = swarm.CreateService(ctx, t, d, swarm.ServiceWithInit(&booleanFalse))
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1), swarm.ServicePoll)
		i = inspectServiceContainer(ctx, t, apiClient, serviceID)
		assert.Check(t, is.Equal(false, *i.HostConfig.Init))
	}
}

func inspectServiceContainer(ctx context.Context, t *testing.T, apiClient client.APIClient, serviceID string) container.InspectResponse {
	t.Helper()
	list, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		Filters: make(client.Filters).Add("label", "com.docker.swarm.service.id="+serviceID),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(list.Items, 1))

	inspect, err := apiClient.ContainerInspect(ctx, list.Items[0].ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	return inspect.Container
}

func TestCreateServiceMultipleTimes(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	overlayName := "overlay1_" + t.Name()
	overlayID := network.CreateNoError(ctx, t, apiClient, overlayName,
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
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, instances), swarm.ServicePoll)

	_, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
	assert.NilError(t, err)

	_, err = apiClient.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
	assert.NilError(t, err)

	poll.WaitOn(t, swarm.NoTasksForService(ctx, apiClient, serviceID), swarm.ServicePoll)

	serviceID2 := swarm.CreateService(ctx, t, d, serviceSpec...)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID2, instances), swarm.ServicePoll)

	_, err = apiClient.ServiceRemove(ctx, serviceID2, client.ServiceRemoveOptions{})
	assert.NilError(t, err)

	// we can't just wait on no tasks for the service, counter-intuitively.
	// Tasks may briefly exist but not show up, if they are in the process
	// of being deallocated. To avoid this case, we should retry network remove
	// a few times, to give tasks time to be deallocated
	poll.WaitOn(t, swarm.NoTasksForService(ctx, apiClient, serviceID2), swarm.ServicePoll)

	for range 5 {
		_, err = apiClient.NetworkRemove(ctx, overlayID, client.NetworkRemoveOptions{})
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

	poll.WaitOn(t, network.IsRemoved(ctx, apiClient, overlayID), poll.WithTimeout(1*time.Minute), poll.WithDelay(10*time.Second))
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
	_, err := c.ServiceCreate(ctx, client.ServiceCreateOptions{
		Spec: spec,
	})
	assert.Check(t, cerrdefs.IsConflict(err))
	assert.ErrorContains(t, err, "service "+serviceName+" already exists")
}

func TestCreateServiceMaxReplicas(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	var maxReplicas uint64 = 2
	serviceSpec := []swarm.ServiceSpecOpt{
		swarm.ServiceWithReplicas(maxReplicas),
		swarm.ServiceWithMaxReplicas(maxReplicas),
	}

	serviceID := swarm.CreateService(ctx, t, d, serviceSpec...)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, maxReplicas), swarm.ServicePoll)

	_, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
}

func TestCreateServiceSecretFileMode(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	secretName := "TestSecret_" + t.Name()
	secretResp, err := apiClient.SecretCreate(ctx, client.SecretCreateOptions{
		Spec: swarmtypes.SecretSpec{
			Annotations: swarmtypes.Annotations{
				Name: secretName,
			},
			Data: []byte("TESTSECRET"),
		},
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

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, instances), swarm.ServicePoll)

	res, err := apiClient.ServiceLogs(ctx, serviceID, client.ServiceLogsOptions{
		Tail:       "1",
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer func() { _ = res.Close() }()

	content, err := io.ReadAll(res)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(content), "-rwxrwxrwx"))

	_, err = apiClient.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, swarm.NoTasksForService(ctx, apiClient, serviceID), swarm.ServicePoll)

	_, err = apiClient.SecretRemove(ctx, secretName, client.SecretRemoveOptions{})
	assert.NilError(t, err)
}

func TestCreateServiceConfigFileMode(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	configName := "TestConfig_" + t.Name()
	resp, err := apiClient.ConfigCreate(ctx, client.ConfigCreateOptions{
		Spec: swarmtypes.ConfigSpec{
			Annotations: swarmtypes.Annotations{
				Name: configName,
			},
			Data: []byte("TESTCONFIG"),
		},
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
			ConfigID:   resp.ID,
			ConfigName: configName,
		}),
	)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, instances))

	res, err := apiClient.ServiceLogs(ctx, serviceID, client.ServiceLogsOptions{
		Tail:       "1",
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer func() { _ = res.Close() }()

	content, err := io.ReadAll(res)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(content), "-rwxrwxrwx"))

	_, err = apiClient.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, swarm.NoTasksForService(ctx, apiClient, serviceID))

	_, err = apiClient.ConfigRemove(ctx, configName, client.ConfigRemoveOptions{})
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
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

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
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, instances))

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
		taskList, err := apiClient.TaskList(ctx, client.TaskListOptions{
			Filters: make(client.Filters).Add("service", serviceID),
		})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(len(taskList.Items), 1))

		// verify that the container has the sysctl option set
		inspect, err := apiClient.ContainerInspect(ctx, taskList.Items[0].Status.ContainerStatus.ContainerID, client.ContainerInspectOptions{})
		assert.NilError(t, err)
		assert.DeepEqual(t, inspect.Container.HostConfig.Sysctls, expectedSysctls)

		// verify that the task has the sysctl option set in the task object
		assert.DeepEqual(t, taskList.Items[0].Spec.ContainerSpec.Sysctls, expectedSysctls)

		// verify that the service also has the sysctl set in the spec.
		result, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
		assert.NilError(t, err)
		assert.DeepEqual(t,
			result.Service.Spec.TaskTemplate.ContainerSpec.Sysctls, expectedSysctls,
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
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	// store the map we're going to be using everywhere.
	capAdd := []string{"CAP_SYS_CHROOT"}
	capDrop := []string{"CAP_NET_RAW"}

	// Create the service with the capabilities options
	var instances uint64 = 1
	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithCapabilities(capAdd, capDrop),
	)

	// wait for the service to converge to 1 running task as expected
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, instances))

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
	taskList, err := apiClient.TaskList(ctx, client.TaskListOptions{
		Filters: make(client.Filters).Add("service", serviceID),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(taskList.Items), 1))

	// verify that the container has the capabilities option set
	inspect, err := apiClient.ContainerInspect(ctx, taskList.Items[0].Status.ContainerStatus.ContainerID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, inspect.Container.HostConfig.CapAdd, capAdd)
	assert.DeepEqual(t, inspect.Container.HostConfig.CapDrop, capDrop)

	// verify that the task has the capabilities option set in the task object
	assert.DeepEqual(t, taskList.Items[0].Spec.ContainerSpec.CapabilityAdd, capAdd)
	assert.DeepEqual(t, taskList.Items[0].Spec.ContainerSpec.CapabilityDrop, capDrop)

	// verify that the service also has the capabilities set in the spec.
	result, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	assert.DeepEqual(t, result.Service.Spec.TaskTemplate.ContainerSpec.CapabilityAdd, capAdd)
	assert.DeepEqual(t, result.Service.Spec.TaskTemplate.ContainerSpec.CapabilityDrop, capDrop)
}

func TestCreateServiceMemorySwap(t *testing.T) {
	ctx := setupTest(t)
	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	toPtr := func(v int64) *int64 { return &v }
	tests := []struct {
		testName string

		swapSpec  *int64
		limitSpec int64

		// as reported by Docker
		expectedDockerSwap int64
	}{
		{
			testName: "default",
		},
		{
			testName:           "memory-limit and memory-swap",
			swapSpec:           toPtr(1 * units.MiB),
			limitSpec:          20 * units.MiB,
			expectedDockerSwap: 21 * units.MiB,
		},
		{
			testName:           "memory-limit alone - should default to twice as much swap",
			limitSpec:          20 * units.MiB,
			expectedDockerSwap: 40 * units.MiB,
		},
		{
			testName:           "memory-limit and zero memory-swap",
			swapSpec:           toPtr(0),
			limitSpec:          20 * units.MiB,
			expectedDockerSwap: 20 * units.MiB,
		},
		{
			testName:           "memory-limit and unlimited memory-swap",
			swapSpec:           toPtr(-1),
			limitSpec:          20 * units.MiB,
			expectedDockerSwap: -1,
		},
	}

	for _, testCase := range tests {
		t.Run("service create with "+testCase.testName, func(t *testing.T) {
			serviceID := swarm.CreateService(
				ctx, t, d,
				swarm.ServiceWithMemorySwap(testCase.swapSpec, testCase.limitSpec),
			)
			poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1))

			inspect, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
			assert.NilError(t, err)

			filter := make(client.Filters)
			filter.Add("service", serviceID)
			tasks, err := apiClient.TaskList(ctx, client.TaskListOptions{
				Filters: filter,
			})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(len(tasks.Items), 1))
			task := tasks.Items[0]

			if testCase.swapSpec == nil {
				assert.Check(t, is.Nil(task.Spec.Resources.SwapBytes))
				assert.Check(t, is.Nil(inspect.Service.Spec.TaskTemplate.Resources.SwapBytes))
			} else {
				assert.Equal(t, *testCase.swapSpec, *task.Spec.Resources.SwapBytes)
				assert.Equal(t, *testCase.swapSpec, *inspect.Service.Spec.TaskTemplate.Resources.SwapBytes)
			}

			// if the host supports it (see https://github.com/moby/moby/blob/v17.03.2-ce/daemon/daemon_unix.go#L290-L294)
			// then check that the swap option is set on the container, and properly reported by the group FS as well
			if testEnv.DaemonInfo.SwapLimit {
				ctr, err := apiClient.ContainerInspect(ctx, task.Status.ContainerStatus.ContainerID, client.ContainerInspectOptions{})
				assert.NilError(t, err)
				assert.Equal(t, testCase.expectedDockerSwap, ctr.Container.HostConfig.Resources.MemorySwap)
			}
		})
	}

	t.Run("cannot create a service with a memory swap option without setting a memory limit", func(t *testing.T) {
		serviceOpts := func(spec *swarmtypes.ServiceSpec) {
			if spec.TaskTemplate.Resources == nil {
				spec.TaskTemplate.Resources = &swarmtypes.ResourceRequirements{}
			}
			spec.TaskTemplate.Resources.SwapBytes = toPtr(10 * units.MiB)
		}

		spec := swarm.CreateServiceSpec(t, serviceOpts)
		_, err := apiClient.ServiceCreate(t.Context(), client.ServiceCreateOptions{
			Spec: spec,
		})

		assert.ErrorContains(t, err, "memory swap provided, but no memory-limit was set")
	})
}

func TestCreateServiceMemorySwappiness(t *testing.T) {
	ctx := setupTest(t)
	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	toPtr := func(v int64) *int64 { return &v }

	tests := []struct {
		testName       string
		swappinessSpec *int64
	}{
		{testName: "default"},
		{testName: "zero memory-swappiness", swappinessSpec: toPtr(0)},
		{testName: "memory-swappiness", swappinessSpec: toPtr(28)},
	}

	for _, testCase := range tests {
		t.Run("service create with "+testCase.testName, func(t *testing.T) {
			serviceID := swarm.CreateService(
				ctx, t, d,
				swarm.ServiceWithMemorySwappiness(testCase.swappinessSpec),
			)
			poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, serviceID, 1))

			filter := make(client.Filters)
			filter.Add("service", serviceID)
			tasks, err := apiClient.TaskList(ctx, client.TaskListOptions{
				Filters: filter,
			})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(len(tasks.Items), 1))
			task := tasks.Items[0]

			inspect, err := apiClient.ServiceInspect(ctx, serviceID, client.ServiceInspectOptions{})
			assert.NilError(t, err)

			// An earlier version of this test also inspected the container
			// created by Swarm to ensure that MemorySwappiness was set on its
			// HostConfig. However, on systems that do not support
			// MemorySwappiness (the Github Actions platform is one, in late
			// 2025), that field in the HostConfig is nilled out, and a warning
			// is returned. Swarm doesn't do anything with the warning, so the
			// setting is silently ignored. Getting the raw SysInfo can show if
			// MemorySwappiness is supported, but that field is not present in
			// a regular Info API call, and so is not part of
			// testEnv.DaemonInfo and cannot be checked (easily) here. So,
			// ultimately, we'll skip that check in the integration test.

			if testCase.swappinessSpec == nil {
				assert.Check(t, is.Nil(task.Spec.Resources.MemorySwappiness))
				assert.Check(t, is.Nil(inspect.Service.Spec.TaskTemplate.Resources.MemorySwappiness))
			} else {
				assert.Equal(t, *testCase.swappinessSpec, *task.Spec.Resources.MemorySwappiness)
				assert.Equal(t, *testCase.swappinessSpec, *inspect.Service.Spec.TaskTemplate.Resources.MemorySwappiness)
			}
		})
	}
}
