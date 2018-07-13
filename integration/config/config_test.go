package config // import "github.com/docker/docker/integration/config"

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestConfigList(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()

	// This test case is ported from the original TestConfigsEmptyList
	configs, err := client.ConfigList(ctx, types.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(configs), 0))

	testName0 := "test0-" + t.Name()
	testName1 := "test1-" + t.Name()
	testNames := []string{testName0, testName1}
	sort.Strings(testNames)

	// create config test0
	createConfig(ctx, t, client, testName0, []byte("TESTINGDATA0"), map[string]string{"type": "test"})

	config1ID := createConfig(ctx, t, client, testName1, []byte("TESTINGDATA1"), map[string]string{"type": "production"})

	names := func(entries []swarmtypes.Config) []string {
		var values []string
		for _, entry := range entries {
			values = append(values, entry.Spec.Name)
		}
		sort.Strings(values)
		return values
	}

	// test by `config ls`
	entries, err := client.ConfigList(ctx, types.ConfigListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(names(entries), testNames))

	testCases := []struct {
		filters  filters.Args
		expected []string
	}{
		// test filter by name `config ls --filter name=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("name", testName0)),
			expected: []string{testName0},
		},
		// test filter by id `config ls --filter id=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("id", config1ID)),
			expected: []string{testName1},
		},
		// test filter by label `config ls --filter label=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("label", "type")),
			expected: testNames,
		},
		{
			filters:  filters.NewArgs(filters.Arg("label", "type=test")),
			expected: []string{testName0},
		},
		{
			filters:  filters.NewArgs(filters.Arg("label", "type=production")),
			expected: []string{testName1},
		},
	}
	for _, tc := range testCases {
		entries, err = client.ConfigList(ctx, types.ConfigListOptions{
			Filters: tc.filters,
		})
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(names(entries), tc.expected))

	}
}

func createConfig(ctx context.Context, t *testing.T, client client.APIClient, name string, data []byte, labels map[string]string) string {
	config, err := client.ConfigCreate(ctx, swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name:   name,
			Labels: labels,
		},
		Data: data,
	})
	assert.NilError(t, err)
	assert.Check(t, config.ID != "")
	return config.ID
}

func TestConfigsCreateAndDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()

	testName := "test_config-" + t.Name()

	// This test case is ported from the original TestConfigsCreate
	configID := createConfig(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	insp, _, err := client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Spec.Name, testName))

	// This test case is ported from the original TestConfigsDelete
	err = client.ConfigRemove(ctx, configID)
	assert.NilError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	assert.Check(t, is.ErrorContains(err, "No such config"))
}

func TestConfigsUpdate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()

	testName := "test_config-" + t.Name()

	// This test case is ported from the original TestConfigsCreate
	configID := createConfig(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	insp, _, err := client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.ID, configID))

	// test UpdateConfig with full ID
	insp.Spec.Labels = map[string]string{"test": "test1"}
	err = client.ConfigUpdate(ctx, configID, insp.Version, insp.Spec)
	assert.NilError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Spec.Labels["test"], "test1"))

	// test UpdateConfig with full name
	insp.Spec.Labels = map[string]string{"test": "test2"}
	err = client.ConfigUpdate(ctx, testName, insp.Version, insp.Spec)
	assert.NilError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Spec.Labels["test"], "test2"))

	// test UpdateConfig with prefix ID
	insp.Spec.Labels = map[string]string{"test": "test3"}
	err = client.ConfigUpdate(ctx, configID[:1], insp.Version, insp.Spec)
	assert.NilError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Spec.Labels["test"], "test3"))

	// test UpdateConfig in updating Data which is not supported in daemon
	// this test will produce an error in func UpdateConfig
	insp.Spec.Data = []byte("TESTINGDATA2")
	err = client.ConfigUpdate(ctx, configID, insp.Version, insp.Spec)
	assert.Check(t, is.ErrorContains(err, "only updates to Labels are allowed"))
}

func TestTemplatedConfig(t *testing.T) {
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()
	ctx := context.Background()

	referencedSecretName := "referencedsecret-" + t.Name()
	referencedSecretSpec := swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: referencedSecretName,
		},
		Data: []byte("this is a secret"),
	}
	referencedSecret, err := client.SecretCreate(ctx, referencedSecretSpec)
	assert.Check(t, err)

	referencedConfigName := "referencedconfig-" + t.Name()
	referencedConfigSpec := swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: referencedConfigName,
		},
		Data: []byte("this is a config"),
	}
	referencedConfig, err := client.ConfigCreate(ctx, referencedConfigSpec)
	assert.Check(t, err)

	templatedConfigName := "templated_config-" + t.Name()
	configSpec := swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name: templatedConfigName,
		},
		Templating: &swarmtypes.Driver{
			Name: "golang",
		},
		Data: []byte("SERVICE_NAME={{.Service.Name}}\n" +
			"{{secret \"referencedsecrettarget\"}}\n" +
			"{{config \"referencedconfigtarget\"}}\n"),
	}

	templatedConfig, err := client.ConfigCreate(ctx, configSpec)
	assert.Check(t, err)

	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithConfig(
			&swarmtypes.ConfigReference{
				File: &swarmtypes.ConfigReferenceFileTarget{
					Name: "/" + templatedConfigName,
					UID:  "0",
					GID:  "0",
					Mode: 0600,
				},
				ConfigID:   templatedConfig.ID,
				ConfigName: templatedConfigName,
			},
		),
		swarm.ServiceWithConfig(
			&swarmtypes.ConfigReference{
				File: &swarmtypes.ConfigReferenceFileTarget{
					Name: "referencedconfigtarget",
					UID:  "0",
					GID:  "0",
					Mode: 0600,
				},
				ConfigID:   referencedConfig.ID,
				ConfigName: referencedConfigName,
			},
		),
		swarm.ServiceWithSecret(
			&swarmtypes.SecretReference{
				File: &swarmtypes.SecretReferenceFileTarget{
					Name: "referencedsecrettarget",
					UID:  "0",
					GID:  "0",
					Mode: 0600,
				},
				SecretID:   referencedSecret.ID,
				SecretName: referencedSecretName,
			},
		),
		swarm.ServiceWithName("svc"),
	)

	var tasks []swarmtypes.Task
	waitAndAssert(t, 60*time.Second, func(t *testing.T) bool {
		tasks = swarm.GetRunningTasks(t, d, serviceID)
		return len(tasks) > 0
	})

	task := tasks[0]
	waitAndAssert(t, 60*time.Second, func(t *testing.T) bool {
		if task.NodeID == "" || (task.Status.ContainerStatus == nil || task.Status.ContainerStatus.ContainerID == "") {
			task, _, _ = client.TaskInspectWithRaw(context.Background(), task.ID)
		}
		return task.NodeID != "" && task.Status.ContainerStatus != nil && task.Status.ContainerStatus.ContainerID != ""
	})

	attach := swarm.ExecTask(t, d, task, types.ExecConfig{
		Cmd:          []string{"/bin/cat", "/" + templatedConfigName},
		AttachStdout: true,
		AttachStderr: true,
	})

	expect := "SERVICE_NAME=svc\n" +
		"this is a secret\n" +
		"this is a config\n"
	assertAttachedStream(t, attach, expect)

	attach = swarm.ExecTask(t, d, task, types.ExecConfig{
		Cmd:          []string{"mount"},
		AttachStdout: true,
		AttachStderr: true,
	})
	assertAttachedStream(t, attach, "tmpfs on /"+templatedConfigName+" type tmpfs")
}

func assertAttachedStream(t *testing.T, attach types.HijackedResponse, expect string) {
	buf := bytes.NewBuffer(nil)
	_, err := stdcopy.StdCopy(buf, buf, attach.Reader)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(buf.String(), expect))
}

func waitAndAssert(t *testing.T, timeout time.Duration, f func(*testing.T) bool) {
	t.Helper()
	after := time.After(timeout)
	for {
		select {
		case <-after:
			t.Fatalf("timed out waiting for condition")
		default:
		}
		if f(t) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestConfigInspect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()

	testName := t.Name()
	configID := createConfig(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	insp, body, err := client.ConfigInspectWithRaw(ctx, configID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(insp.Spec.Name, testName))

	var config swarmtypes.Config
	err = json.Unmarshal(body, &config)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(config, insp))
}
