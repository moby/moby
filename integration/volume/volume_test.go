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
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/request"
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
	created, err := apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name: name,
	})
	assert.NilError(t, err)
	namedV := created.Volume

	expected := volume.Volume{
		// Ignore timestamp of CreatedAt
		CreatedAt:  namedV.CreatedAt,
		Driver:     "local",
		Scope:      "local",
		Name:       name,
		Mountpoint: filepath.Join(testEnv.DaemonInfo.DockerRootDir, "volumes", name, "_data"),
	}
	assert.Check(t, is.DeepEqual(namedV, expected, cmpopts.EquateEmpty()))

	res, err := apiClient.VolumeList(ctx, client.VolumeListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(res.Items) > 0)

	volumes := res.Items[:0]
	for _, v := range res.Items {
		if v.Name == namedV.Name {
			volumes = append(volumes, v)
		}
	}

	assert.Check(t, is.Equal(len(volumes), 1))
	assert.Check(t, is.DeepEqual(volumes[0], expected, cmpopts.EquateEmpty()))
}

func TestVolumesRemove(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	id := container.Create(ctx, t, apiClient, container.WithVolume(prefix+slash+"foo"))

	inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	vname := inspect.Container.Mounts[0].Name

	t.Run("volume in use", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, vname, client.VolumeRemoveOptions{})
		assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "volume is in use"))
	})

	t.Run("volume not in use", func(t *testing.T) {
		_, err = apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
			Force: true,
		})
		assert.NilError(t, err)

		_, err = apiClient.VolumeRemove(ctx, vname, client.VolumeRemoveOptions{})
		assert.NilError(t, err)
	})

	t.Run("non-existing volume", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, "no_such_volume", client.VolumeRemoveOptions{})
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})

	t.Run("non-existing volume force", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, "no_such_volume", client.VolumeRemoveOptions{Force: true})
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

	inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	vname := inspect.Container.Mounts[0].Name

	t.Run("volume in use", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, vname, client.VolumeRemoveOptions{})
		assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "volume is in use"))
	})

	t.Run("volume not in use", func(t *testing.T) {
		_, err = apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
			Force: true,
		})
		assert.NilError(t, err)

		_, err = apiClient.VolumeRemove(ctx, vname, client.VolumeRemoveOptions{})
		assert.NilError(t, err)
	})

	t.Run("non-existing volume", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, "no_such_volume", client.VolumeRemoveOptions{})
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})

	t.Run("non-existing volume force", func(t *testing.T) {
		_, err = apiClient.VolumeRemove(ctx, "no_such_volume", client.VolumeRemoveOptions{Force: true})
		assert.NilError(t, err)
	})
}

func TestVolumesInspect(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	now := time.Now()
	created, err := apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{})
	assert.NilError(t, err)
	v := created.Volume

	res, err := apiClient.VolumeInspect(ctx, v.Name, client.VolumeInspectOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.DeepEqual(res.Volume, v, cmpopts.EquateEmpty()))

	// comparing CreatedAt field time for the new volume to now. Truncate to 1 minute precision to avoid false positive
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(res.Volume.CreatedAt))
	assert.NilError(t, err)
	assert.Check(t, createdAt.Unix()-now.Unix() < 60, "CreatedAt (%s) exceeds creation time (%s) 60s", createdAt, now)

	// update atime and mtime for the "_data" directory (which would happen during volume initialization)
	modifiedAt := time.Now().Local().Add(5 * time.Hour)
	err = os.Chtimes(res.Volume.Mountpoint, modifiedAt, modifiedAt)
	assert.NilError(t, err)

	res, err = apiClient.VolumeInspect(ctx, v.Name, client.VolumeInspectOptions{})
	assert.NilError(t, err)

	createdAt2, err := time.Parse(time.RFC3339, strings.TrimSpace(res.Volume.CreatedAt))
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
	created, err := apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{})
	assert.NilError(t, err)
	anonV := created.Volume

	// Create a named volume
	created, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name: "test",
	})
	assert.NilError(t, err)
	namedV := created.Volume

	// Prune anonymous volumes
	prune, err := apiClient.VolumesPrune(ctx, client.VolumePruneOptions{})
	assert.NilError(t, err)
	report := prune.Report
	assert.Check(t, is.Equal(len(report.VolumesDeleted), 1))
	assert.Check(t, is.Equal(report.VolumesDeleted[0], anonV.Name))

	_, err = apiClient.VolumeInspect(ctx, namedV.Name, client.VolumeInspectOptions{})
	assert.NilError(t, err)

	// Prune all volumes
	_, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{})
	assert.NilError(t, err)
	prune, err = apiClient.VolumesPrune(ctx, client.VolumePruneOptions{
		All: true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(prune.Report.VolumesDeleted), 2))

	// Create a named volume and an anonymous volume, and prune all
	_, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{})
	assert.NilError(t, err)
	_, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{Name: "test"})
	assert.NilError(t, err)

	prune, err = apiClient.VolumesPrune(ctx, client.VolumePruneOptions{
		All: true,
	})

	assert.NilError(t, err)
	report = prune.Report
	assert.Check(t, is.Equal(len(report.VolumesDeleted), 2))

	// Validate that older API versions still have the old behavior of pruning all local volumes
	clientOld, err := client.New(client.FromEnv, client.WithVersion("1.41"))
	assert.NilError(t, err)
	defer clientOld.Close()
	assert.Equal(t, clientOld.ClientVersion(), "1.41")

	created, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{})
	assert.NilError(t, err)
	anonV = created.Volume
	created, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{Name: "test-api141"})
	assert.NilError(t, err)
	namedV = created.Volume

	prune, err = clientOld.VolumesPrune(ctx, client.VolumePruneOptions{})
	assert.NilError(t, err)
	report = prune.Report
	assert.Check(t, is.Equal(len(report.VolumesDeleted), 2))
	assert.Check(t, is.Contains(report.VolumesDeleted, anonV.Name))
	assert.Check(t, is.Contains(report.VolumesDeleted, namedV.Name))
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
	defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{})

	inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	assert.Assert(t, is.Len(inspect.Container.Mounts, 1))

	volumeName := inspect.Container.Mounts[0].Name
	assert.Assert(t, volumeName != "")

	_, err = apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{})
	assert.NilError(t, err)

	res, err := apiClient.VolumesPrune(ctx, client.VolumePruneOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Contains(res.Report.VolumesDeleted, volumeName))
}
