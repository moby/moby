package build

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/v2/testutil/fakecontext"
	"gotest.tools/v3/assert"
)

// Do builds an image from the given context and returns the image ID.
func Do(ctx context.Context, t *testing.T, apiClient client.APIClient, buildCtx *fakecontext.Fake) string {
	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), build.ImageBuildOptions{})
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	assert.NilError(t, err)
	img := GetImageIDFromBody(t, resp.Body)
	t.Cleanup(func() {
		apiClient.ImageRemove(ctx, img, client.ImageRemoveOptions{Force: true})
	})
	return img
}

// GetImageIDFromBody reads the image ID from the build response body.
func GetImageIDFromBody(t *testing.T, body io.Reader) string {
	var id string
	dec := json.NewDecoder(body)
	for {
		var jm jsonmessage.JSONMessage
		err := dec.Decode(&jm)
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		if jm.Aux == nil {
			continue
		}

		var br build.Result
		if err := json.Unmarshal(*jm.Aux, &br); err == nil {
			if br.ID == "" {
				continue
			}
			id = br.ID
			break
		}
	}
	_, _ = io.Copy(io.Discard, body)

	assert.Assert(t, id != "", "could not read image ID from build output")
	return id
}
