package volume

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumesCreateAndList(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	name := t.Name()
	// Windows file system is case insensitive
	if testEnv.OSType == "windows" {
		name = strings.ToLower(name)
	}
	vol, err := client.VolumeCreate(ctx, volume.VolumeCreateBody{
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

	volList, err := client.VolumeList(ctx, filters.Args{})
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
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	id := container.Create(ctx, t, client, container.WithVolume(prefix+slash+"foo"))

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
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	now := time.Now()
	vol, err := client.VolumeCreate(ctx, volume.VolumeCreateBody{})
	assert.NilError(t, err)

	inspected, err := client.VolumeInspect(ctx, vol.Name)
	assert.NilError(t, err)

	assert.Check(t, is.DeepEqual(inspected, vol, cmpopts.EquateEmpty()))

	// comparing CreatedAt field time for the new volume to now. Truncate to 1 minute precision to avoid false positive
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(inspected.CreatedAt))
	assert.NilError(t, err)
	assert.Check(t, createdAt.Unix()-now.Unix() < 60, "CreatedAt (%s) exceeds creation time (%s) 60s", createdAt, now)
}

// TestVolumesInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestVolumesInvalidJSON(t *testing.T) {
	defer setupTest(t)()

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{"/volumes/create"}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()

			t.Run("invalid content type", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("{}"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString(`{} trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.OSType == "windows" {
		return "c:", `\`
	}
	return "", "/"
}
