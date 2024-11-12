package image

import (
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45732
func TestPruneDontDeleteUsedDangling(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	client := d.NewClientT(t)
	defer client.Close()

	danglingID := specialimage.Load(ctx, t, client, specialimage.Dangling)

	_, _, err := client.ImageInspectWithRaw(ctx, danglingID)
	assert.NilError(t, err, "Test dangling image doesn't exist")

	container.Create(ctx, t, client,
		container.WithImage(danglingID),
		container.WithCmd("sleep", "60"))

	pruned, err := client.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	assert.NilError(t, err)

	for _, deleted := range pruned.ImagesDeleted {
		if strings.Contains(deleted.Deleted, danglingID) || strings.Contains(deleted.Untagged, danglingID) {
			t.Errorf("used dangling image %s shouldn't be deleted", danglingID)
		}
	}

	_, _, err = client.ImageInspectWithRaw(ctx, danglingID)
	assert.NilError(t, err, "Test dangling image should still exist")
}

func TestPruneLexographicalOrder(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	d.LoadBusybox(ctx, t)

	inspect, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
	assert.NilError(t, err)

	id := inspect.ID

	tags := []string{"h", "a", "j", "o", "s", "q", "w", "e", "r", "t"}
	for _, tag := range tags {
		err = apiClient.ImageTag(ctx, id, "busybox:"+tag)
		assert.NilError(t, err)
	}
	err = apiClient.ImageTag(ctx, id, "busybox:z")
	assert.NilError(t, err)

	_, err = apiClient.ImageRemove(ctx, "busybox:latest", image.RemoveOptions{Force: true})
	assert.NilError(t, err)

	// run container
	cid := container.Create(ctx, t, apiClient, container.WithImage(id))
	defer container.Remove(ctx, t, apiClient, cid, containertypes.RemoveOptions{Force: true})

	pruned, err := apiClient.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "false")))
	assert.NilError(t, err)

	assert.Check(t, is.Len(pruned.ImagesDeleted, len(tags)))
	for _, p := range pruned.ImagesDeleted {
		assert.Check(t, is.Equal(p.Deleted, ""))
		assert.Check(t, p.Untagged != "busybox:z")
	}
}

// Regression test for https://github.com/moby/moby/issues/48063
func TestPruneDontDeleteUsedImage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")

	ctx := setupTest(t)

	for _, env := range []struct {
		name    string
		prepare func(t *testing.T, client *daemon.Daemon, apiClient *client.Client) error
		check   func(t *testing.T, apiClient *client.Client, pruned image.PruneReport)
	}{
		{
			// Container uses the busybox:latest image and it's the only image
			// tag with the same target.
			name: "single tag",
			check: func(t *testing.T, apiClient *client.Client, pruned image.PruneReport) {
				assert.Check(t, is.Len(pruned.ImagesDeleted, 0))

				_, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
				assert.NilError(t, err, "Busybox image should still exist")
			},
		},
		{
			// Container uses the busybox:latest image and there's also a second
			// busybox:other tag pointing to the same image.
			name: "two tags",
			prepare: func(t *testing.T, d *daemon.Daemon, apiClient *client.Client) error {
				return apiClient.ImageTag(ctx, "busybox:latest", "busybox:a")
			},
			check: func(t *testing.T, apiClient *client.Client, pruned image.PruneReport) {
				if assert.Check(t, is.Len(pruned.ImagesDeleted, 1)) {
					assert.Check(t, is.Equal(pruned.ImagesDeleted[0].Deleted, ""))
					assert.Check(t, is.Equal(pruned.ImagesDeleted[0].Untagged, "busybox:a"))
				}

				_, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:a")
				assert.Check(t, err != nil, "Busybox:a image should be deleted")

				_, _, err = apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
				assert.Check(t, err == nil, "Busybox:latest image should still exist")
			},
		},
	} {
		for _, tc := range []struct {
			name    string
			imageID func(t *testing.T, inspect image.InspectResponse) string
		}{
			{
				name: "full id",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					return inspect.ID
				},
			},
			{
				name: "full id without sha256 prefix",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					return strings.TrimPrefix(inspect.ID, "sha256:")
				},
			},
			{
				name: "truncated id (without sha256 prefix)",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					return strings.TrimPrefix(inspect.ID, "sha256:")[:8]
				},
			},
			{
				name: "repo and digest without tag",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					skip.If(t, !testEnv.UsingSnapshotter())

					return "busybox@" + inspect.ID
				},
			},
			{
				name: "tagged and digested",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					skip.If(t, !testEnv.UsingSnapshotter())

					return "busybox:latest@" + inspect.ID
				},
			},
			{
				name: "repo digest",
				imageID: func(t *testing.T, inspect image.InspectResponse) string {
					// graphdriver won't have a repo digest
					skip.If(t, len(inspect.RepoDigests) == 0, "no repo digest")

					return inspect.RepoDigests[0]
				},
			},
		} {
			t.Run(env.name+"/"+tc.name, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				d := daemon.New(t)
				d.Start(t)
				defer d.Stop(t)

				apiClient := d.NewClientT(t)
				defer apiClient.Close()

				d.LoadBusybox(ctx, t)

				if env.prepare != nil {
					err := env.prepare(t, d, apiClient)
					assert.NilError(t, err, "prepare failed")
				}

				inspect, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
				assert.NilError(t, err)

				image := tc.imageID(t, inspect)
				t.Log(image)

				cid := container.Run(ctx, t, apiClient,
					container.WithImage(image),
					container.WithCmd("sleep", "60"))
				defer container.Remove(ctx, t, apiClient, cid, containertypes.RemoveOptions{Force: true})

				// dangling=false also prunes unused images
				pruned, err := apiClient.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "false")))
				assert.NilError(t, err)

				env.check(t, apiClient, pruned)
			})
		}
	}
}
