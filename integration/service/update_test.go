package service // import "github.com/docker/docker/integration/service"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
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
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar"}))

	// add label to non-empty set
	service.Spec.Labels["foo2"] = "bar"
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar", "foo2": "bar"}))

	delete(service.Spec.Labels, "foo2")
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{"foo": "bar"}))

	delete(service.Spec.Labels, "foo")
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
	service = getService(t, cli, serviceID)
	assert.Check(t, is.DeepEqual(service.Spec.Labels, map[string]string{}))

	// now make sure we can add again
	service.Spec.Labels["foo"] = "bar"
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)
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
			},
			SecretID:   resp.ID,
			SecretName: secretName,
		},
	)
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)

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
			},
			ConfigID:   resp.ID,
			ConfigName: configName,
		},
	)
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	assert.NilError(t, err)

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
	service = getService(t, cli, serviceID)
	assert.Check(t, is.Equal(0, len(service.Spec.TaskTemplate.ContainerSpec.Configs)))

	err = cli.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

func getService(t *testing.T, cli client.ServiceAPIClient, serviceID string) swarmtypes.Service {
	t.Helper()
	service, _, err := cli.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)
	return service
}
