package config

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/internal/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestConfigInspect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	testName := t.Name()
	configID := createConfig(ctx, t, c, testName, []byte("TESTINGDATA"), nil)

	result, err := c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(result.Config.Spec.Name, testName))

	var config swarmtypes.Config
	err = json.Unmarshal(result.Raw, &config)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(config, result.Config))
}

func TestConfigList(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// This test case is ported from the original TestConfigsEmptyList
	result, err := c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(result.Items), 0))

	testName0 := "test0-" + t.Name()
	testName1 := "test1-" + t.Name()
	testNames := []string{testName0, testName1}
	sort.Strings(testNames)

	// create config test0
	createConfig(ctx, t, c, testName0, []byte("TESTINGDATA0"), map[string]string{"type": "test"})

	config1ID := createConfig(ctx, t, c, testName1, []byte("TESTINGDATA1"), map[string]string{"type": "production"})

	// test by `config ls`
	res, err := c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(configNamesFromList(res.Items), testNames))

	testCases := []struct {
		desc     string
		filters  client.Filters
		expected []string
	}{
		{
			desc:     "test filter by name",
			filters:  make(client.Filters).Add("name", testName0),
			expected: []string{testName0},
		},
		{
			desc:     "test filter by id",
			filters:  make(client.Filters).Add("id", config1ID),
			expected: []string{testName1},
		},
		{
			desc:     "test filter by label key only",
			filters:  make(client.Filters).Add("label", "type"),
			expected: testNames,
		},
		{
			desc:     "test filter by label key=value " + testName0,
			filters:  make(client.Filters).Add("label", "type=test"),
			expected: []string{testName0},
		},
		{
			desc:     "test filter by label key=value " + testName1,
			filters:  make(client.Filters).Add("label", "type=production"),
			expected: []string{testName1},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			res, err = c.ConfigList(ctx, client.ConfigListOptions{
				Filters: tc.filters,
			})
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(configNamesFromList(res.Items), tc.expected))
		})
	}
}

func createConfig(ctx context.Context, t *testing.T, apiClient client.APIClient, name string, data []byte, labels map[string]string) string {
	result, err := apiClient.ConfigCreate(ctx, client.ConfigCreateOptions{
		Spec: swarmtypes.ConfigSpec{
			Annotations: swarmtypes.Annotations{
				Name:   name,
				Labels: labels,
			},
			Data: data,
		},
	})
	assert.NilError(t, err)
	assert.Check(t, result.ID != "")
	return result.ID
}

func TestConfigsCreateAndDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	testName := "test_config-" + t.Name()
	configID := createConfig(ctx, t, c, testName, []byte("TESTINGDATA"), nil)

	_, err := c.ConfigRemove(ctx, configID, client.ConfigRemoveOptions{})
	assert.NilError(t, err)

	_, err = c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.Check(t, cerrdefs.IsNotFound(err))
	assert.Check(t, is.ErrorContains(err, configID))

	_, err = c.ConfigRemove(ctx, "non-existing", client.ConfigRemoveOptions{})
	assert.Check(t, cerrdefs.IsNotFound(err))
	assert.Check(t, is.ErrorContains(err, "non-existing"))

	testName = "test_secret_with_labels_" + t.Name()
	configID = createConfig(ctx, t, c, testName, []byte("TESTINGDATA"), map[string]string{
		"key1": "value1",
		"key2": "value2",
	})

	result, err := c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(result.Config.Spec.Name, testName))
	assert.Check(t, is.Equal(len(result.Config.Spec.Labels), 2))
	assert.Check(t, is.Equal(result.Config.Spec.Labels["key1"], "value1"))
	assert.Check(t, is.Equal(result.Config.Spec.Labels["key2"], "value2"))
}

func TestConfigsUpdate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	testName := "test_config-" + t.Name()
	configID := createConfig(ctx, t, c, testName, []byte("TESTINGDATA"), nil)

	insp, err := c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Config.ID, configID))

	// test UpdateConfig with full ID
	insp.Config.Spec.Labels = map[string]string{"test": "test1"}
	_, err = c.ConfigUpdate(ctx, configID, client.ConfigUpdateOptions{Version: insp.Config.Version, Spec: insp.Config.Spec})
	assert.NilError(t, err)

	insp, err = c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Config.Spec.Labels["test"], "test1"))

	// test UpdateConfig with full name
	insp.Config.Spec.Labels = map[string]string{"test": "test2"}
	_, err = c.ConfigUpdate(ctx, testName, client.ConfigUpdateOptions{Version: insp.Config.Version, Spec: insp.Config.Spec})
	assert.NilError(t, err)

	insp, err = c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Config.Spec.Labels["test"], "test2"))

	// test UpdateConfig with prefix ID
	insp.Config.Spec.Labels = map[string]string{"test": "test3"}
	_, err = c.ConfigUpdate(ctx, configID[:1], client.ConfigUpdateOptions{Version: insp.Config.Version, Spec: insp.Config.Spec})
	assert.NilError(t, err)

	insp, err = c.ConfigInspect(ctx, configID, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Config.Spec.Labels["test"], "test3"))

	// test UpdateConfig in updating Data which is not supported in daemon
	// this test will produce an error in func UpdateConfig
	insp.Config.Spec.Data = []byte("TESTINGDATA2")
	_, err = c.ConfigUpdate(ctx, configID, client.ConfigUpdateOptions{Version: insp.Config.Version, Spec: insp.Config.Spec})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	assert.Check(t, is.ErrorContains(err, "only updates to Labels are allowed"))
}

func TestTemplatedConfig(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := testutil.StartSpan(baseContext, t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	referencedSecretName := "referencedsecret-" + t.Name()
	referencedSecretSpec := swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: referencedSecretName,
		},
		Data: []byte("this is a secret"),
	}
	referencedSecret, err := c.SecretCreate(ctx, client.SecretCreateOptions{
		Spec: referencedSecretSpec,
	})
	assert.Check(t, err)

	referencedConfigName := "referencedconfig-" + t.Name()
	referencedConfigSpec := swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: referencedConfigName,
		},
		Data: []byte("this is a config"),
	}
	referencedConfigResult, err := c.ConfigCreate(ctx, client.ConfigCreateOptions{
		Spec: referencedConfigSpec,
	})
	assert.Check(t, err)

	templatedConfigName := "templated_config-" + t.Name()
	configSpec := swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: templatedConfigName,
		},
		Templating: &swarmtypes.Driver{
			Name: "golang",
		},
		Data: []byte(`SERVICE_NAME={{.Service.Name}}
{{secret "referencedsecrettarget"}}
{{config "referencedconfigtarget"}}
`),
	}

	templatedConfigResult, err := c.ConfigCreate(ctx, client.ConfigCreateOptions{
		Spec: configSpec,
	})
	assert.Check(t, err)

	const serviceName = "svc_templated_config"
	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithConfig(
			&swarmtypes.ConfigReference{
				File: &swarmtypes.ConfigReferenceFileTarget{
					Name: "templated_config",
					UID:  "0",
					GID:  "0",
					Mode: 0o600,
				},
				ConfigID:   templatedConfigResult.ID,
				ConfigName: templatedConfigName,
			},
		),
		swarm.ServiceWithConfig(
			&swarmtypes.ConfigReference{
				File: &swarmtypes.ConfigReferenceFileTarget{
					Name: "referencedconfigtarget",
					UID:  "0",
					GID:  "0",
					Mode: 0o600,
				},
				ConfigID:   referencedConfigResult.ID,
				ConfigName: referencedConfigName,
			},
		),
		swarm.ServiceWithSecret(
			&swarmtypes.SecretReference{
				File: &swarmtypes.SecretReferenceFileTarget{
					Name: "referencedsecrettarget",
					UID:  "0",
					GID:  "0",
					Mode: 0o600,
				},
				SecretID:   referencedSecret.ID,
				SecretName: referencedSecretName,
			},
		),
		swarm.ServiceWithName(serviceName),
	)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll, poll.WithTimeout(1*time.Minute))

	tasks := swarm.GetRunningTasks(ctx, t, c, serviceID)
	assert.Assert(t, len(tasks) > 0, "no running tasks found for service %s", serviceID)

	resp := swarm.ExecTask(ctx, t, d, tasks[0], client.ExecCreateOptions{
		Cmd:          []string{"/bin/cat", "/templated_config"},
		AttachStdout: true,
		AttachStderr: true,
	})

	const expect = "SERVICE_NAME=" + serviceName + "\nthis is a secret\nthis is a config\n"
	var outBuf, errBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(outBuf.String(), expect))
	assert.Check(t, is.Equal(errBuf.String(), ""))

	outBuf.Reset()
	errBuf.Reset()
	resp = swarm.ExecTask(ctx, t, d, tasks[0], client.ExecCreateOptions{
		Cmd:          []string{"mount"},
		AttachStdout: true,
		AttachStderr: true,
	})

	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(outBuf.String(), "tmpfs on /templated_config type tmpfs"), "expected to be mounted as tmpfs")
	assert.Check(t, is.Equal(errBuf.String(), ""))
}

// Test case for 28884
func TestConfigCreateResolve(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	configName := "test_config_" + t.Name()
	configID := createConfig(ctx, t, c, configName, []byte("foo"), nil)

	fakeName := configID
	fakeID := createConfig(ctx, t, c, fakeName, []byte("fake foo"), nil)

	res, err := c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Contains(configNamesFromList(res.Items), configName))
	assert.Assert(t, is.Contains(configNamesFromList(res.Items), fakeName))

	_, err = c.ConfigRemove(ctx, configID, client.ConfigRemoveOptions{})
	assert.NilError(t, err)

	// Fake one will remain
	res, err = c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.DeepEqual(configNamesFromList(res.Items), []string{fakeName}))

	// Remove based on name prefix of the fake one
	// (which is the same as the ID of foo one) should not work
	// as search is only done based on:
	// - Full ID
	// - Full Name
	// - Partial ID (prefix)
	_, err = c.ConfigRemove(ctx, configID[:5], client.ConfigRemoveOptions{})
	assert.Assert(t, err != nil)
	res, err = c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.DeepEqual(configNamesFromList(res.Items), []string{fakeName}))

	// Remove based on ID prefix of the fake one should succeed
	_, err = c.ConfigRemove(ctx, fakeID[:5], client.ConfigRemoveOptions{})
	assert.NilError(t, err)
	res, err = c.ConfigList(ctx, client.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(0, len(res.Items)))
}

func configNamesFromList(entries []swarmtypes.Config) []string {
	var values []string
	for _, entry := range entries {
		values = append(values, entry.Spec.Name)
	}
	sort.Strings(values)
	return values
}
