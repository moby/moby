package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestResize(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	t.Run("success", func(t *testing.T) {
		cID := container.Run(ctx, t, apiClient, container.WithTty(true))
		defer container.Remove(ctx, t, apiClient, cID, containertypes.RemoveOptions{Force: true})
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
		cID := container.Run(ctx, t, apiClient)
		defer container.Remove(ctx, t, apiClient, cID, containertypes.RemoveOptions{Force: true})

		const valueNotSet = "unset"

		tests := []struct {
			doc, height, width, expErr string
		}{
			{
				doc:    "unset height",
				height: valueNotSet,
				width:  "100",
				expErr: `invalid resize height "": invalid syntax`,
			},
			{
				doc:    "unset width",
				height: "100",
				width:  valueNotSet,
				expErr: `invalid resize width "": invalid syntax`,
			},
			{
				doc:    "empty height",
				width:  "100",
				expErr: `invalid resize height "": invalid syntax`,
			},
			{
				doc:    "empty width",
				height: "100",
				expErr: `invalid resize width "": invalid syntax`,
			},
			{
				doc:    "non-numeric height",
				height: "not-a-number",
				width:  "100",
				expErr: `invalid resize height "not-a-number": invalid syntax`,
			},
			{
				doc:    "non-numeric width",
				height: "100",
				width:  "not-a-number",
				expErr: `invalid resize width "not-a-number": invalid syntax`,
			},
			{
				doc:    "negative height",
				height: "-100",
				width:  "100",
				expErr: `invalid resize height "-100": value out of range`,
			},
			{
				doc:    "negative width",
				height: "100",
				width:  "-100",
				expErr: `invalid resize width "-100": value out of range`,
			},
			{
				doc:    "out of range height",
				height: "4294967296", // math.MaxUint32+1
				width:  "100",
				expErr: `invalid resize height "4294967296": value out of range`,
			},
			{
				doc:    "out of range width",
				height: "100",
				width:  "4294967296", // math.MaxUint32+1
				expErr: `invalid resize width "4294967296": value out of range`,
			},
		}
		for _, tc := range tests {
			t.Run(tc.doc, func(t *testing.T) {
				// Manually creating a request here, as the APIClient would invalidate
				// these values before they're sent.
				vals := url.Values{}
				if tc.height != valueNotSet {
					vals.Add("h", tc.height)
				}
				if tc.width != valueNotSet {
					vals.Add("w", tc.width)
				}
				res, _, err := req.Post(ctx, "/containers/"+cID+"/resize?"+vals.Encode())
				assert.NilError(t, err)
				assert.Check(t, is.Equal(http.StatusBadRequest, res.StatusCode))

				var errorResponse types.ErrorResponse
				err = json.NewDecoder(res.Body).Decode(&errorResponse)
				assert.NilError(t, err)
				assert.Check(t, is.ErrorContains(errorResponse, tc.expErr))
			})
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		cID := container.Create(ctx, t, apiClient, container.WithCmd("echo"))
		defer container.Remove(ctx, t, apiClient, cID, containertypes.RemoveOptions{Force: true})
		err := apiClient.ContainerResize(ctx, cID, containertypes.ResizeOptions{
			Height: 40,
			Width:  40,
		})
		assert.Check(t, is.ErrorType(err, errdefs.IsConflict))
		assert.Check(t, is.ErrorContains(err, "is not running"))
	})
}
