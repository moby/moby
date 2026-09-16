package build

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
)

// Do builds an image from the given context with the supplied options
// and returns the image ID.
func Do(ctx context.Context, t *testing.T, apiClient client.APIClient, buildCtx *fakecontext.Fake, options client.ImageBuildOptions) string {
	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), options)
	assert.NilError(t, err)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	img := GetImageIDFromBody(t, resp.Body)
	t.Cleanup(func() {
		_, _ = apiClient.ImageRemove(ctx, img, client.ImageRemoveOptions{Force: true})
	})
	return img
}

// GetImageIDFromBody reads the image ID from the build response body.
func GetImageIDFromBody(t *testing.T, body io.Reader) string {
	t.Helper()

	// Buffer log output from DisplayStream callbacks. Calling t.Log from inside the
	// callbacks would attribute the output to jsonmessage rather than the test,
	// despite this function being marked with t.Helper().
	var logs []string
	Log := func(msg string) {
		logs = append(logs, msg)
	}

	var id string
	err := jsonmessage.DisplayStream(body, io.Discard,
		jsonmessage.WithAuxCallback(func(jm jsonstream.Message) {
			t.Helper()

			switch jm.ID {
			case "moby.buildkit.trace":
				var dt []byte
				if err := json.Unmarshal(*jm.Aux, &dt); err != nil {
					Log("Error unmarshalling buildkit trace " + err.Error())
					return
				}

				var sr controlapi.StatusResponse
				if err := proto.Unmarshal(dt, &sr); err != nil {
					Log("Error unmarshalling buildkit trace proto" + err.Error())
					return
				}

				for _, vtx := range sr.GetVertexes() {
					Log(vtx.String())
				}
				for _, vtx := range sr.GetStatuses() {
					Log(vtx.String())
				}
				for _, vtx := range sr.GetLogs() {
					Log(vtx.String())
				}
				for _, vtx := range sr.GetWarnings() {
					Log(vtx.String())
				}

			case "moby.image.id":
				fallthrough
			default:
				var br build.Result
				if err := json.Unmarshal(*jm.Aux, &br); err == nil && br.ID != "" {
					id = br.ID
					return
				}
				Log("Raw Aux " + string(*jm.Aux))
			}
		}),
		jsonmessage.WithMessagePrinter(func(jm jsonstream.Message, _ io.Writer) error {
			if jm.Error != nil {
				return jm.Error
			}
			if jm.Progress != nil && (jm.Progress.Current > 0 || jm.Progress.Total > 0) {
				return nil
			}
			msg := jm.Stream
			if msg == "" {
				msg = jm.Status
			}
			if jm.ID != "" {
				msg = jm.ID + ": " + msg
			}
			Log(msg)
			return nil
		}),
	)
	for _, msg := range logs {
		t.Log(msg)
	}
	assert.NilError(t, err)
	assert.Assert(t, id != "", "could not read image ID from build output")
	return id
}
