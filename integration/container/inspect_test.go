package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
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

// TestNetworkAliasesAreEmpty verifies that network-scoped aliases are not set
// for non-custom networks (network-scoped aliases are only supported for
// custom networks, except for the "Default Switch" network on Windows).
func TestNetworkAliasesAreEmpty(t *testing.T) {
	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t)

	netModes := []string{"host", "bridge", "none"}
	if runtime.GOOS == "windows" {
		netModes = []string{"nat", "none"}
	}

	for _, nwMode := range netModes {
		t.Run(nwMode, func(t *testing.T) {
			ctr := container.Create(ctx, t, apiClient,
				container.WithName("ctr-"+nwMode),
				container.WithImage("busybox:latest"),
				container.WithNetworkMode(nwMode))
			defer apiClient.ContainerRemove(ctx, ctr, containertypes.RemoveOptions{
				Force: true,
			})

			inspect := container.Inspect(ctx, t, apiClient, ctr)
			netAliases := inspect.NetworkSettings.Networks[nwMode].Aliases

			assert.Check(t, is.Nil(netAliases))
		})
	}
}
