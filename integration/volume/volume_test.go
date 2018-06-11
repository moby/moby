package volume

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestVolumesCreateAndList(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	name := t.Name()
	vol, err := client.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Name: name,
	})
	assert.NilError(t, err)

	expected := types.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  vol.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Mountpoint: fmt.Sprintf("%s/volumes/%s/_data", testEnv.DaemonInfo.DockerRootDir, name),
	}
	assert.Check(t, is.DeepEqual(vol, expected, cmpopts.EquateEmpty()))

	volumes, err := client.VolumeList(ctx, filters.Args{})
	assert.NilError(t, err)

	assert.Check(t, is.Equal(len(volumes.Volumes), 1))
	assert.Check(t, volumes.Volumes[0] != nil)
	assert.Check(t, is.DeepEqual(*volumes.Volumes[0], expected, cmpopts.EquateEmpty()))
}

func TestVolumesRemove(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	id := container.Create(t, ctx, client, container.WithVolume(prefix+slash+"foo"))

	c, err := client.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	vname := c.Mounts[0].Name

	err = client.VolumeRemove(ctx, vname, false)
	assert.Check(t, is.ErrorContains(err, "volume is in use"))

	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force: true,
	})
	assert.NilError(t, err)

	err = client.VolumeRemove(ctx, vname, false)
	assert.NilError(t, err)
}

func TestVolumesInspect(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	// sampling current time minus a minute so to now have false positive in case of delays
	now := time.Now().Truncate(time.Minute)

	name := t.Name()
	_, err := client.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Name: name,
	})
	assert.NilError(t, err)

	vol, err := client.VolumeInspect(ctx, name)
	assert.NilError(t, err)

	expected := types.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  vol.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Mountpoint: fmt.Sprintf("%s/volumes/%s/_data", testEnv.DaemonInfo.DockerRootDir, name),
	}
	assert.Check(t, is.DeepEqual(vol, expected, cmpopts.EquateEmpty()))

	// comparing CreatedAt field time for the new volume to now. Removing a minute from both to avoid false positive
	testCreatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(vol.CreatedAt))
	assert.NilError(t, err)
	testCreatedAt = testCreatedAt.Truncate(time.Minute)
	assert.Check(t, is.Equal(testCreatedAt.Equal(now), true), "Time Volume is CreatedAt not equal to current time")
}

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.OSType == "windows" {
		return "c:", `\`
	}
	return "", "/"
}
