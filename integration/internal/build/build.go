package build

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/v2/testutil/fakecontext"
)

// Do builds an image from the given context and returns the image ID.
func Do(ctx context.Context, t *testing.T, client client.APIClient, buildCtx *fakecontext.Fake) string {
	resp, err := client.ImageBuild(ctx, buildCtx.AsTarReader(t), build.ImageBuildOptions{})
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	assert.NilError(t, err)
	img := GetImageIDFromBody(t, resp.Body)
	t.Cleanup(func() {
		client.ImageRemove(ctx, img, image.RemoveOptions{Force: true})
	})
	return img
}

// GetImageIDFromBody reads the image ID from the build response body.
func GetImageIDFromBody(t *testing.T, body io.Reader) string {
	var (
		jm  jsonmessage.JSONMessage
		br  build.Result
		dec = json.NewDecoder(body)
	)
	for {
		err := dec.Decode(&jm)
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		if jm.Aux == nil {
			continue
		}
		assert.NilError(t, json.Unmarshal(*jm.Aux, &br))
		assert.Assert(t, br.ID != "", "could not read image ID from build output")
		break
	}
	io.Copy(io.Discard, body)
	return br.ID
}
