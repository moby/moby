package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/docker/docker/pkg/stringid"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test simulates the scenario mentioned in #31392:
// Having two linked container, renaming the target and bringing a replacement
// and then deleting and recreating the source container linked to the new target.
// This checks that "rename" updates source container correctly and doesn't set it to null.
func TestRenameLinkedContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	aID := container.Run(t, ctx, client, container.WithName("a0"))
	bID := container.Run(t, ctx, client, container.WithName("b0"), container.WithLinks("a0"))

	err := client.ContainerRename(ctx, aID, "a1")
	require.NoError(t, err)

	container.Run(t, ctx, client, container.WithName("a0"))

	err = client.ContainerRemove(ctx, bID, types.ContainerRemoveOptions{Force: true})
	require.NoError(t, err)

	bID = container.Run(t, ctx, client, container.WithName("b0"), container.WithLinks("a0"))

	inspect, err := client.ContainerInspect(ctx, bID)
	require.NoError(t, err)
	assert.Equal(t, []string{"/a0:/b0/a0"}, inspect.HostConfig.Links)
}

func TestRenameStoppedContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	oldName := "first_name"
	cID := container.Run(t, ctx, client, container.WithName(oldName), container.WithCmd("sh"))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.Name, "/"+oldName)

	newName := "new_name" + stringid.GenerateNonCryptoID()
	err = client.ContainerRename(ctx, oldName, newName)
	require.NoError(t, err)

	inspect, err = client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.Name, "/"+newName)
}

func TestRenameRunningContainerAndReuse(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	oldName := "first_name"
	cID := container.Run(t, ctx, client, container.WithName(oldName))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	newName := "new_name" + stringid.GenerateNonCryptoID()
	err := client.ContainerRename(ctx, oldName, newName)
	require.NoError(t, err)

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.Name, "/"+newName)

	_, err = client.ContainerInspect(ctx, oldName)
	testutil.ErrorContains(t, err, "No such container: "+oldName)

	cID = container.Run(t, ctx, client, container.WithName(oldName))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	inspect, err = client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.Name, "/"+oldName)
}

func TestRenameInvalidName(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	oldName := "first_name"
	cID := container.Run(t, ctx, client, container.WithName(oldName))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerRename(ctx, oldName, "new:invalid")
	testutil.ErrorContains(t, err, "Invalid container name")

	inspect, err := client.ContainerInspect(ctx, oldName)
	require.NoError(t, err)
	assert.Equal(t, inspect.ID, cID)
}

// Test case for GitHub issue 22466
// Docker's service discovery works for named containers so
// ping to a named container should work, and an anonymous
// container without a name does not work with service discovery.
// However, an anonymous could be renamed to a named container.
// This test is to make sure once the container has been renamed,
// the service discovery for the (re)named container works.
func TestRenameAnonymousContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	_, err := client.NetworkCreate(ctx, "network1", types.NetworkCreate{})
	require.NoError(t, err)
	cID := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			"network1": {},
		}
		c.HostConfig.NetworkMode = "network1"
	})
	err = client.ContainerRename(ctx, cID, "container1")
	require.NoError(t, err)
	err = client.ContainerStart(ctx, "container1", types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	count := "-c"
	if testEnv.OSType == "windows" {
		count = "-n"
	}
	cID = container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			"network1": {},
		}
		c.HostConfig.NetworkMode = "network1"
	}, container.WithCmd("ping", count, "1", "container1"))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, inspect.State.ExitCode, 0)
}

// TODO: should be a unit test
func TestRenameContainerWithSameName(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client, container.WithName("old"))
	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	err := client.ContainerRename(ctx, "old", "old")
	testutil.ErrorContains(t, err, "Renaming a container with the same name")
	err = client.ContainerRename(ctx, cID, "old")
	testutil.ErrorContains(t, err, "Renaming a container with the same name")
}

// Test case for GitHub issue 23973
// When a container is being renamed, the container might
// be linked to another container. In that case, the meta data
// of the linked container should be updated so that the other
// container could still reference to the container that is renamed.
func TestRenameContainerWithLinkedContainer(t *testing.T) {
	skip.If(t, !testEnv.IsLocalDaemon())

	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	db1ID := container.Run(t, ctx, client, container.WithName("db1"))
	poll.WaitOn(t, containerIsInState(ctx, client, db1ID, "running"), poll.WithDelay(100*time.Millisecond))

	app1ID := container.Run(t, ctx, client, container.WithName("app1"), container.WithLinks("db1:/mysql"))
	poll.WaitOn(t, containerIsInState(ctx, client, app1ID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerRename(ctx, "app1", "app2")
	require.NoError(t, err)

	inspect, err := client.ContainerInspect(ctx, "app2/mysql")
	require.NoError(t, err)
	assert.Equal(t, inspect.ID, db1ID)
}
