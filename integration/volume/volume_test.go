package volume

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/google/go-cmp/cmp/cmpopts"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
	"github.com/moby/moby/v2/testutil/fakecontext"
	"github.com/moby/moby/v2/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestVolumesCreateAndList(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	name := t.Name()
	// Windows file system is case insensitive
	if testEnv.DaemonInfo.OSType == "windows" {
		name = strings.ToLower(name)
	}
	vol, err := apiClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	assert.NilError(t, err)

	expected := volume.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  vol.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Mountpoint: filepath.Join(testEnv.DaemonInfo.DockerRootDir, "volumes", name, "_data"),
	}
	assert.Check(t, is.DeepEqual(vol, expected, cmpopts.EquateEmpty()))

	volList, err := apiClient.VolumeList(ctx, client.VolumeListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(volList.Volumes) > 0)

	volumes := volList.Volumes[:0]
	for _, v := range volList.Volumes {
		if v.Name == vol.Name {
			volumes = append(volumes, v)
		}
	}

	assert.Check(t, is.Equal(len(volumes), 1))
	assert.Check(t, volumes[0] != nil)
	assert.Check(t, is.DeepEqual(*volumes[0], expected, cmpopts.EquateEmpty()))
}

func TestVolumesRemove(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	id := container.Create(ctx, t, apiClient, container.WithVolume(prefix+slash+"foo"))

	c, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	vname := c.Mounts[0].Name

	t.Run("volume in use", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, vname, false)
		assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "volume is in use"))
	})

	t.Run("volume not in use", func(t *testing.T) {
		err = apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{
			Force: true,
		})
		assert.NilError(t, err)

		err = apiClient.VolumeRemove(ctx, vname, false)
		assert.NilError(t, err)
	})

	t.Run("non-existing volume", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, "no_such_volume", false)
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})

	t.Run("non-existing volume force", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, "no_such_volume", true)
		assert.NilError(t, err)
	})
}

// TestVolumesRemoveSwarmEnabled tests that an error is returned if a volume
// is in use, also if swarm is enabled (and cluster volumes are supported).
//
// Regression test for https://github.com/docker/cli/issues/4082
func TestVolumesRemoveSwarmEnabled(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "TODO enable on windows")
	ctx := setupTest(t)

	t.Parallel()

	// Spin up a new daemon, so that we can run this test in parallel (it's a slow test)
	d := daemon.New(t)
	d.StartAndSwarmInit(ctx, t)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	id := container.Create(ctx, t, apiClient, container.WithVolume(prefix+slash+"foo"))

	c, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	vname := c.Mounts[0].Name

	t.Run("volume in use", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, vname, false)
		assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "volume is in use"))
	})

	t.Run("volume not in use", func(t *testing.T) {
		err = apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{
			Force: true,
		})
		assert.NilError(t, err)

		err = apiClient.VolumeRemove(ctx, vname, false)
		assert.NilError(t, err)
	})

	t.Run("non-existing volume", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, "no_such_volume", false)
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})

	t.Run("non-existing volume force", func(t *testing.T) {
		err = apiClient.VolumeRemove(ctx, "no_such_volume", true)
		assert.NilError(t, err)
	})
}

func TestVolumesInspect(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	now := time.Now()
	vol, err := apiClient.VolumeCreate(ctx, volume.CreateOptions{})
	assert.NilError(t, err)

	inspected, err := apiClient.VolumeInspect(ctx, vol.Name)
	assert.NilError(t, err)

	assert.Check(t, is.DeepEqual(inspected, vol, cmpopts.EquateEmpty()))

	// comparing CreatedAt field time for the new volume to now. Truncate to 1 minute precision to avoid false positive
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(inspected.CreatedAt))
	assert.NilError(t, err)
	assert.Check(t, createdAt.Unix()-now.Unix() < 60, "CreatedAt (%s) exceeds creation time (%s) 60s", createdAt, now)

	// update atime and mtime for the "_data" directory (which would happen during volume initialization)
	modifiedAt := time.Now().Local().Add(5 * time.Hour)
	err = os.Chtimes(inspected.Mountpoint, modifiedAt, modifiedAt)
	assert.NilError(t, err)

	inspected, err = apiClient.VolumeInspect(ctx, vol.Name)
	assert.NilError(t, err)

	createdAt2, err := time.Parse(time.RFC3339, strings.TrimSpace(inspected.CreatedAt))
	assert.NilError(t, err)

	// Check that CreatedAt didn't change after updating atime and mtime of the "_data" directory
	// Related issue: #38274
	assert.Equal(t, createdAt, createdAt2)
}

// TestVolumesInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestVolumesInvalidJSON(t *testing.T) {
	ctx := setupTest(t)

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{"/volumes/create"}

	for _, ep := range endpoints {
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)

			t.Run("invalid content type", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{}"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString(`{} trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ctx, ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.DaemonInfo.OSType == "windows" {
		return "c:", `\`
	}
	return "", "/"
}

func TestVolumePruneAnonymous(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	// Create an anonymous volume
	v, err := apiClient.VolumeCreate(ctx, volume.CreateOptions{})
	assert.NilError(t, err)

	// Create a named volume
	vNamed, err := apiClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: "test",
	})
	assert.NilError(t, err)

	// Prune anonymous volumes
	pruneReport, err := apiClient.VolumesPrune(ctx, filters.Args{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(pruneReport.VolumesDeleted), 1))
	assert.Check(t, is.Equal(pruneReport.VolumesDeleted[0], v.Name))

	_, err = apiClient.VolumeInspect(ctx, vNamed.Name)
	assert.NilError(t, err)

	// Prune all volumes
	_, err = apiClient.VolumeCreate(ctx, volume.CreateOptions{})
	assert.NilError(t, err)

	pruneReport, err = apiClient.VolumesPrune(ctx, filters.NewArgs(filters.Arg("all", "1")))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(pruneReport.VolumesDeleted), 2))

	// Validate that older API versions still have the old behavior of pruning all local volumes
	clientOld, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.41"))
	assert.NilError(t, err)
	defer clientOld.Close()
	assert.Equal(t, clientOld.ClientVersion(), "1.41")

	v, err = apiClient.VolumeCreate(ctx, volume.CreateOptions{})
	assert.NilError(t, err)
	vNamed, err = apiClient.VolumeCreate(ctx, volume.CreateOptions{Name: "test-api141"})
	assert.NilError(t, err)

	pruneReport, err = clientOld.VolumesPrune(ctx, filters.Args{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(pruneReport.VolumesDeleted), 2))
	assert.Check(t, is.Contains(pruneReport.VolumesDeleted, v.Name))
	assert.Check(t, is.Contains(pruneReport.VolumesDeleted, vNamed.Name))
}

func TestVolumePruneAnonFromImage(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	volDest := "/foo"
	if testEnv.DaemonInfo.OSType == "windows" {
		volDest = `c:\\foo`
	}

	dockerfile := `FROM busybox
VOLUME ` + volDest

	img := build.Do(ctx, t, apiClient, fakecontext.New(t, "", fakecontext.WithDockerfile(dockerfile)))

	id := container.Create(ctx, t, apiClient, container.WithImage(img))
	defer apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{})

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)

	assert.Assert(t, is.Len(inspect.Mounts, 1))

	volumeName := inspect.Mounts[0].Name
	assert.Assert(t, volumeName != "")

	err = apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{})
	assert.NilError(t, err)

	pruneReport, err := apiClient.VolumesPrune(ctx, filters.Args{})
	assert.NilError(t, err)
	assert.Assert(t, is.Contains(pruneReport.VolumesDeleted, volumeName))
}
