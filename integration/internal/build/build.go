package build

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
)

// Do builds an image from the given context and returns the image ID.
func Do(ctx context.Context, t *testing.T, apiClient client.APIClient, buildCtx *fakecontext.Fake) string {
	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), client.ImageBuildOptions{})
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
	buf := bytes.NewBuffer(nil)
	dec := json.NewDecoder(body)
	for {
		var jm jsonmessage.JSONMessage
		err := dec.Decode(&jm)
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)

		if handled := processBuildkitAux(t, &jm, &id); handled {
			continue
		}

		buf.Reset()
		jm.Display(buf, false)
		if buf.Len() == 0 {
			continue
		}

		t.Log(buf.String())

		if jm.Aux == nil {
			continue
		}

		var br build.Result
		if err := json.Unmarshal(*jm.Aux, &br); err == nil {
			if br.ID == "" {
				continue
			}
			id = br.ID
			continue
		}

		t.Log("Raw Aux", string(*jm.Aux))
	}
	_, _ = io.Copy(io.Discard, body)

	assert.Assert(t, id != "", "could not read image ID from build output")
	return id
}

func processBuildkitAux(t *testing.T, jm *jsonmessage.JSONMessage, id *string) bool {
	if jm.ID == "moby.buildkit.trace" {
		var dt []byte
		if err := json.Unmarshal(*jm.Aux, &dt); err != nil {
			t.Log("Error unmarshalling buildkit trace", err)
			return true
		}
		var sr controlapi.StatusResponse
		if err := proto.Unmarshal(dt, &sr); err != nil {
			t.Log("Error unmarshalling buildkit trace proto", err)
			return true
		}
		for _, vtx := range sr.GetVertexes() {
			t.Log(vtx.String())
		}
		for _, vtx := range sr.GetStatuses() {
			t.Log(vtx.String())
		}
		for _, vtx := range sr.GetLogs() {
			t.Log(vtx.String())
		}
		for _, vtx := range sr.GetWarnings() {
			t.Log(vtx.String())
		}
		return true
	}
	if jm.ID == "moby.image.id" {
		var br build.Result
		if err := json.Unmarshal(*jm.Aux, &br); err == nil {
			*id = br.ID
			return true
		}
	}
	return false
}
