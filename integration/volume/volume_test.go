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
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVolumesCreateAndList(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	name := t.Name()
	vol, err := client.VolumeCreate(ctx, volumetypes.VolumesCreateBody{
		Name: name,
	})
	require.NoError(t, err)

	expected := types.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  vol.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Options:    map[string]string{},
		Mountpoint: fmt.Sprintf("%s/volumes/%s/_data", testEnv.DaemonInfo.DockerRootDir, name),
	}
	assert.Equal(t, vol, expected)

	volumes, err := client.VolumeList(ctx, filters.Args{})
	require.NoError(t, err)

	assert.Equal(t, len(volumes.Volumes), 1)
	assert.NotNil(t, volumes.Volumes[0])
	assert.Equal(t, *volumes.Volumes[0], expected)
}

func TestVolumesRemove(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	id := container.Create(t, ctx, client, container.WithVolume(prefix+"foo"))

	c, err := client.ContainerInspect(ctx, id)
	require.NoError(t, err)
	vname := c.Mounts[0].Name

	err = client.VolumeRemove(ctx, vname, false)
	testutil.ErrorContains(t, err, "volume is in use")

	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force: true,
	})
	require.NoError(t, err)

	err = client.VolumeRemove(ctx, vname, false)
	require.NoError(t, err)
}

func TestVolumesInspect(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	// sampling current time minus a minute so to now have false positive in case of delays
	now := time.Now().Truncate(time.Minute)

	name := t.Name()
	_, err := client.VolumeCreate(ctx, volumetypes.VolumesCreateBody{
		Name: name,
	})
	require.NoError(t, err)

	vol, err := client.VolumeInspect(ctx, name)
	require.NoError(t, err)

	expected := types.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  vol.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Options:    map[string]string{},
		Mountpoint: fmt.Sprintf("%s/volumes/%s/_data", testEnv.DaemonInfo.DockerRootDir, name),
	}
	assert.Equal(t, vol, expected)

	// comparing CreatedAt field time for the new volume to now. Removing a minute from both to avoid false positive
	testCreatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(vol.CreatedAt))
	require.NoError(t, err)
	testCreatedAt = testCreatedAt.Truncate(time.Minute)
	assert.Equal(t, testCreatedAt.Equal(now), true, "Time Volume is CreatedAt not equal to current time")
}

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.OSType == "windows" {
		return "c:", `\`
	}
	return "", "/"
}
