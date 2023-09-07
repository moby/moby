package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestInspectCpusetInConfigPre120(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" || !testEnv.DaemonInfo.CPUSet)

	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t, client.WithVersion("1.19"))

	name := strings.ToLower(t.Name())
	// Create container with up to-date-API
	container.Run(ctx, t, request.NewAPIClient(t), container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Resources.CpusetCpus = "0"
		},
	)
	poll.WaitOn(t, container.IsInState(ctx, apiClient, name, "exited"), poll.WithDelay(100*time.Millisecond))

	_, body, err := apiClient.ContainerInspectWithRaw(ctx, name, false)
	assert.NilError(t, err)

	var inspectJSON map[string]interface{}
	err = json.Unmarshal(body, &inspectJSON)
	assert.NilError(t, err, "unable to unmarshal body for version 1.19: %s", err)

	config, ok := inspectJSON["Config"]
	assert.Check(t, is.Equal(true, ok), "Unable to find 'Config'")

	cfg := config.(map[string]interface{})
	_, ok = cfg["Cpuset"]
	assert.Check(t, is.Equal(true, ok), "API version 1.19 expected to include Cpuset in 'Config'")
}

func TestInspectAnnotations(t *testing.T) {
	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t)

	annotations := map[string]string{
		"hello": "world",
		"foo":   "bar",
	}

	name := strings.ToLower(t.Name())
	id := container.Create(ctx, t, apiClient,
		container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Annotations = annotations
		},
	)

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(inspect.HostConfig.Annotations, annotations))
}
