package container // import "github.com/docker/docker/integration/container"

import (
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stringid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// This test simulates the scenario mentioned in #31392:
// Having two linked container, renaming the target and bringing a replacement
// and then deleting and recreating the source container linked to the new target.
// This checks that "rename" updates source container correctly and doesn't set it to null.
func TestRenameLinkedContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	aName := "a0" + t.Name()
	bName := "b0" + t.Name()
	aID := container.Run(ctx, t, apiClient, container.WithName(aName))
	bID := container.Run(ctx, t, apiClient, container.WithName(bName), container.WithLinks(aName))

	err := apiClient.ContainerRename(ctx, aID, "a1"+t.Name())
	assert.NilError(t, err)

	container.Run(ctx, t, apiClient, container.WithName(aName))

	err = apiClient.ContainerRemove(ctx, bID, containertypes.RemoveOptions{Force: true})
	assert.NilError(t, err)

	bID = container.Run(ctx, t, apiClient, container.WithName(bName), container.WithLinks(aName))

	inspect, err := apiClient.ContainerInspect(ctx, bID)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual([]string{"/" + aName + ":/" + bName + "/" + aName}, inspect.HostConfig.Links))
}

func TestRenameStoppedContainer(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	oldName := "first_name" + t.Name()
	cID := container.Run(ctx, t, apiClient, container.WithName(oldName), container.WithCmd("sh"))

	inspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("/"+oldName, inspect.Name))

	newName := "new_name" + stringid.GenerateRandomID()
	err = apiClient.ContainerRename(ctx, oldName, newName)
	assert.NilError(t, err)

	inspect, err = apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("/"+newName, inspect.Name))
}

func TestRenameRunningContainerAndReuse(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	oldName := "first_name" + t.Name()
	cID := container.Run(ctx, t, apiClient, container.WithName(oldName))

	newName := "new_name" + stringid.GenerateRandomID()
	err := apiClient.ContainerRename(ctx, oldName, newName)
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("/"+newName, inspect.Name))

	_, err = apiClient.ContainerInspect(ctx, oldName)
	assert.Check(t, is.ErrorContains(err, "No such container: "+oldName))

	cID = container.Run(ctx, t, apiClient, container.WithName(oldName))

	inspect, err = apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("/"+oldName, inspect.Name))
}

func TestRenameInvalidName(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	oldName := "first_name" + t.Name()
	cID := container.Run(ctx, t, apiClient, container.WithName(oldName))

	err := apiClient.ContainerRename(ctx, oldName, "new:invalid")
	assert.Check(t, is.ErrorContains(err, "Invalid container name"))

	inspect, err := apiClient.ContainerInspect(ctx, oldName)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(cID, inspect.ID))
}

// Test case for GitHub issue 22466
// Docker's service discovery works for named containers so
// ping to a named container should work, and an anonymous
// container without a name does not work with service discovery.
// However, an anonymous could be renamed to a named container.
// This test is to make sure once the container has been renamed,
// the service discovery for the (re)named container works.
func TestRenameAnonymousContainer(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	networkName := "network1" + t.Name()
	_, err := apiClient.NetworkCreate(ctx, networkName, network.CreateOptions{})

	assert.NilError(t, err)
	cID := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			networkName: {},
		}
		c.HostConfig.NetworkMode = containertypes.NetworkMode(networkName)
	})

	container1Name := "container1" + t.Name()
	err = apiClient.ContainerRename(ctx, cID, container1Name)
	assert.NilError(t, err)
	// Stop/Start the container to get registered
	// FIXME(vdemeester) this is a really weird behavior as it fails otherwise
	err = apiClient.ContainerStop(ctx, container1Name, containertypes.StopOptions{})
	assert.NilError(t, err)
	err = apiClient.ContainerStart(ctx, container1Name, containertypes.StartOptions{})
	assert.NilError(t, err)

	count := "-c"
	if testEnv.DaemonInfo.OSType == "windows" {
		count = "-n"
	}
	cID = container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			networkName: {},
		}
		c.HostConfig.NetworkMode = containertypes.NetworkMode(networkName)
	}, container.WithCmd("ping", count, "1", container1Name))
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(0, inspect.State.ExitCode), "container %s exited with the wrong exitcode: %s", cID, inspect.State.Error)
}

// TODO: should be a unit test
func TestRenameContainerWithSameName(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	oldName := "old" + t.Name()
	cID := container.Run(ctx, t, apiClient, container.WithName(oldName))
	err := apiClient.ContainerRename(ctx, oldName, oldName)
	assert.Check(t, is.ErrorContains(err, "Renaming a container with the same name"))
	err = apiClient.ContainerRename(ctx, cID, oldName)
	assert.Check(t, is.ErrorContains(err, "Renaming a container with the same name"))
}

// Test case for GitHub issue 23973
// When a container is being renamed, the container might
// be linked to another container. In that case, the meta data
// of the linked container should be updated so that the other
// container could still reference to the container that is renamed.
func TestRenameContainerWithLinkedContainer(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	db1Name := "db1" + t.Name()
	db1ID := container.Run(ctx, t, apiClient, container.WithName(db1Name))

	app1Name := "app1" + t.Name()
	app2Name := "app2" + t.Name()
	container.Run(ctx, t, apiClient, container.WithName(app1Name), container.WithLinks(db1Name+":/mysql"))

	err := apiClient.ContainerRename(ctx, app1Name, app2Name)
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerInspect(ctx, app2Name+"/mysql")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(db1ID, inspect.ID))
}

// Regression test for https://github.com/moby/moby/issues/47186
func TestRenameContainerTwice(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	ctrName := "c0"
	container.Run(ctx, t, apiClient, container.WithName("c0"))
	defer func() {
		container.Remove(ctx, t, apiClient, ctrName, containertypes.RemoveOptions{
			Force: true,
		})
	}()

	err := apiClient.ContainerRename(ctx, "c0", "c1")
	assert.NilError(t, err)
	ctrName = "c1"

	err = apiClient.ContainerRename(ctx, "c1", "c2")
	assert.NilError(t, err)
	ctrName = "c2"
}
