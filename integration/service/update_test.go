package service // import "github.com/docker/docker/integration/service"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestServiceUpdateLabel(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	cli := d.NewClientT(t)
	defer cli.Close()

	ctx := context.Background()
	serviceName := "TestService_" + t.Name()
	serviceID := swarm.CreateService(t, d, swarm.ServiceWithName(serviceName))
	service := getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{}))

	// add label to empty set
	service.Spec.Labels["foo"] = "bar"
	_, err := cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceSpecIsUpdated(cli, serviceID, service.Version.Index), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar"}))

	// add label to non-empty set
	service.Spec.Labels["foo2"] = "bar"
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceSpecIsUpdated(cli, serviceID, service.Version.Index), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar", "foo2": "bar"}))

	delete(service.Spec.Labels, "foo2")
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceSpecIsUpdated(cli, serviceID, service.Version.Index), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar"}))

	delete(service.Spec.Labels, "foo")
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceSpecIsUpdated(cli, serviceID, service.Version.Index), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{}))

	// now make sure we can add again
	service.Spec.Labels["foo"] = "bar"
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceSpecIsUpdated(cli, serviceID, service.Version.Index), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar"}))

	err = cli.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

func TestServiceUpdateSecrets(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	cli := d.NewClientT(t)
	defer cli.Close()

	ctx := context.Background()
	secretName := "TestSecret_" + t.Name()
	secretTarget := "targetName"
	resp, err := cli.SecretCreate(ctx, swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: secretName,
		},
		Data: []byte("TESTINGDATA"),
	})
	assert.NilError(t, err)
	assert.Check(t, resp.ID != "")

	serviceName := "TestService_" + t.Name()
	serviceID := swarm.CreateService(t, d, swarm.ServiceWithName(serviceName))
	service := getService(t, cli, serviceID)

	// add secret
	service.Spec.TaskTemplate.ContainerSpec.Secrets = append(service.Spec.TaskTemplate.ContainerSpec.Secrets,
		&swarmtypes.SecretReference{
			File: &swarmtypes.SecretReferenceFileTarget{
				Name: secretTarget,
				UID:  "0",
				GID:  "0",
				Mode: 0o600,
			},
			SecretID:   resp.ID,
			SecretName: secretName,
		},
	)
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)

	service = getService(t, cli, serviceID)
	secrets := service.Spec.TaskTemplate.ContainerSpec.Secrets
	assert.Assert(t, is.Equal(1, len(secrets)))

	secret := *secrets[0]
	assert.Check(t, is.Equal(secretName, secret.SecretName))
	assert.Check(t, nil != secret.File)
	assert.Check(t, is.Equal(secretTarget, secret.File.Name))

	// remove
	service.Spec.TaskTemplate.ContainerSpec.Secrets = []*swarmtypes.SecretReference{}
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.Equal(0, len(service.Spec.TaskTemplate.ContainerSpec.Secrets)))

	err = cli.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

func TestServiceUpdateConfigs(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	cli := d.NewClientT(t)
	defer cli.Close()

	ctx := context.Background()
	configName := "TestConfig_" + t.Name()
	configTarget := "targetName"
	resp, err := cli.ConfigCreate(ctx, swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: configName,
		},
		Data: []byte("TESTINGDATA"),
	})
	assert.NilError(t, err)
	assert.Check(t, resp.ID != "")

	serviceName := "TestService_" + t.Name()
	serviceID := swarm.CreateService(t, d, swarm.ServiceWithName(serviceName))
	service := getService(t, cli, serviceID)

	// add config
	service.Spec.TaskTemplate.ContainerSpec.Configs = append(service.Spec.TaskTemplate.ContainerSpec.Configs,
		&swarmtypes.ConfigReference{
			File: &swarmtypes.ConfigReferenceFileTarget{
				Name: configTarget,
				UID:  "0",
				GID:  "0",
				Mode: 0o600,
			},
			ConfigID:   resp.ID,
			ConfigName: configName,
		},
	)
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)

	service = getService(t, cli, serviceID)
	configs := service.Spec.TaskTemplate.ContainerSpec.Configs
	assert.Assert(t, is.Equal(1, len(configs)))

	config := *configs[0]
	assert.Check(t, is.Equal(configName, config.ConfigName))
	assert.Check(t, nil != config.File)
	assert.Check(t, is.Equal(configTarget, config.File.Name))

	// remove
	service.Spec.TaskTemplate.ContainerSpec.Configs = []*swarmtypes.ConfigReference{}
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.Equal(0, len(service.Spec.TaskTemplate.ContainerSpec.Configs)))

	err = cli.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

func TestServiceUpdateNetwork(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	cli := d.NewClientT(t)
	defer cli.Close()

	ctx := context.Background()

	// Create a overlay network
	testNet := "testNet" + t.Name()
	overlayID := network.CreateNoError(ctx, t, cli, testNet,
		network.WithDriver("overlay"))

	var instances uint64 = 1
	// Create service with the overlay network
	serviceName := "TestServiceUpdateNetworkRM_" + t.Name()
	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(testNet))

	poll.WaitOn(t, swarm.RunningTasksCount(cli, serviceID, instances), swarm.ServicePoll)
	service := getService(t, cli, serviceID)
	netInfo, err := cli.NetworkInspect(ctx, testNet, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})
	assert.NilError(t, err)
	assert.Assert(t, len(netInfo.Containers) == 2, "Expected 2 endpoints, one for container and one for LB Sandbox")

	// Remove network from service
	service.Spec.TaskTemplate.Networks = []swarmtypes.NetworkAttachmentConfig{}
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)

	netInfo, err = cli.NetworkInspect(ctx, testNet, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})

	assert.NilError(t, err)
	assert.Assert(t, len(netInfo.Containers) == 0, "Load balancing endpoint still exists in network")

	err = cli.NetworkRemove(ctx, overlayID)
	assert.NilError(t, err)

	err = cli.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

// TestServiceUpdatePidsLimit tests creating and updating a service with PidsLimit
func TestServiceUpdatePidsLimit(t *testing.T) {
	skip.If(
		t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.41"),
		"setting pidslimit for services is not supported before api v1.41",
	)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	tests := []struct {
		name      string
		pidsLimit int64
		expected  int64
	}{
		{
			name:      "create service with PidsLimit 300",
			pidsLimit: 300,
			expected:  300,
		},
		{
			name:      "unset PidsLimit to 0",
			pidsLimit: 0,
			expected:  0,
		},
		{
			name:      "update PidsLimit to 100",
			pidsLimit: 100,
			expected:  100,
		},
	}

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	cli := d.NewClientT(t)
	defer func() { _ = cli.Close() }()
	ctx := context.Background()
	var (
		serviceID string
		service   swarmtypes.Service
	)
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if i == 0 {
				serviceID = swarm.CreateService(t, d, swarm.ServiceWithPidsLimit(tc.pidsLimit))
			} else {
				service = getService(t, cli, serviceID)
				if service.Spec.TaskTemplate.Resources == nil {
					service.Spec.TaskTemplate.Resources = &swarmtypes.ResourceRequirements{}
				}
				if service.Spec.TaskTemplate.Resources.Limits == nil {
					service.Spec.TaskTemplate.Resources.Limits = &swarmtypes.Limit{}
				}
				service.Spec.TaskTemplate.Resources.Limits.Pids = tc.pidsLimit
				_, err := cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
				assert.NilError(t, err)
				poll.WaitOn(t, serviceIsUpdated(cli, serviceID), swarm.ServicePoll)
			}

			poll.WaitOn(t, swarm.RunningTasksCount(cli, serviceID, 1), swarm.ServicePoll)
			service = getService(t, cli, serviceID)
			container := getServiceTaskContainer(ctx, t, cli, serviceID)
			assert.Equal(t, service.Spec.TaskTemplate.Resources.Limits.Pids, tc.expected)
			if tc.expected == 0 {
				if container.HostConfig.Resources.PidsLimit != nil {
					t.Fatalf("Expected container.HostConfig.Resources.PidsLimit to be nil")
				}
			} else {
				assert.Assert(t, container.HostConfig.Resources.PidsLimit != nil)
				assert.Equal(t, *container.HostConfig.Resources.PidsLimit, tc.expected)
			}
		})
	}

	err := cli.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)
}

func getServiceTaskContainer(ctx context.Context, t *testing.T, cli client.APIClient, serviceID string) types.ContainerJSON {
	t.Helper()
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(
			filters.Arg("service", serviceID),
			filters.Arg("desired-state", "running"),
		),
	})
	assert.NilError(t, err)
	assert.Assert(t, len(tasks) > 0)

	ctr, err := cli.ContainerInspect(ctx, tasks[0].Status.ContainerStatus.ContainerID)
	assert.NilError(t, err)
	assert.Equal(t, ctr.State.Running, true)
	return ctr
}

func getService(t *testing.T, cli client.ServiceAPIClient, serviceID string) swarmtypes.Service {
	t.Helper()
	service, _, err := cli.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)
	return service
}

func serviceIsUpdated(client client.ServiceAPIClient, serviceID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		service, _, err := client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
		switch {
		case err != nil:
			return poll.Error(err)
		case service.UpdateStatus != nil && service.UpdateStatus.State == swarmtypes.UpdateStateCompleted:
			return poll.Success()
		default:
			if service.UpdateStatus != nil {
				return poll.Continue("waiting for service %s to be updated, state: %s, message: %s", serviceID, service.UpdateStatus.State, service.UpdateStatus.Message)
			}
			return poll.Continue("waiting for service %s to be updated", serviceID)
		}
	}
}

func serviceSpecIsUpdated(client client.ServiceAPIClient, serviceID string, serviceOldVersion uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		service, _, err := client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
		switch {
		case err != nil:
			return poll.Error(err)
		case service.Version.Index > serviceOldVersion:
			return poll.Success()
		default:
			return poll.Continue("waiting for service %s to be updated", serviceID)
		}
	}
}
