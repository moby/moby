package container // import "github.com/docker/docker/integration/container"

import (
	"net/http"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestResize(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	t.Run("success", func(t *testing.T) {
		cID := container.Run(ctx, t, apiClient, container.WithTty(true))
		err := apiClient.ContainerResize(ctx, cID, containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.NilError(t, err)
		// TODO(thaJeztah): also check if the resize happened
		//
		// Note: container inspect shows the initial size that was
		// set when creating the container. Actual resize happens in
		// containerd, and currently does not update the container's
		// config after running (but does send a "resize" event).
	})

	t.Run("invalid size", func(t *testing.T) {
		skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.32"), "broken in earlier versions")
		cID := container.Run(ctx, t, apiClient)

		// Manually creating a request here, as the APIClient would invalidate
		// these values before they're sent.
		res, _, err := req.Post(ctx, "/containers/"+cID+"/resize?h=foo&w=bar")
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(http.StatusBadRequest, res.StatusCode))
	})

	t.Run("invalid state", func(t *testing.T) {
		cID := container.Create(ctx, t, apiClient, container.WithCmd("echo"))
		err := apiClient.ContainerResize(ctx, cID, containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.Check(t, is.ErrorType(err, errdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "is not running"))
	})
}
