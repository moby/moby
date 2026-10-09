package system

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	iimage "github.com/moby/moby/v2/integration/internal/image"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/request"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestPruneProgress(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "requires a local test daemon")

	ctx := testutil.StartSpan(baseContext, t)
	d := daemon.New(t, daemon.WithContainerdSocket(""))
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	imageID := iimage.Load(ctx, t, apiClient, specialimage.Dangling)
	containerID := container.Create(ctx, t, apiClient, container.WithImage(imageID))
	_, err := apiClient.NetworkCreate(ctx, "prune-progress-network", client.NetworkCreateOptions{})
	assert.NilError(t, err)
	_, err = apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{Name: "prune-progress-volume"})
	assert.NilError(t, err)

	// Populate BuildKit cache so the test exercises successful cache removals.
	source := fakecontext.New(t, "", fakecontext.WithDockerfile("FROM scratch\nCOPY payload /payload\n"), fakecontext.WithFile("payload", "prune progress"))
	defer source.Close()
	built, err := apiClient.ImageBuild(ctx, source.AsTarReader(t), client.ImageBuildOptions{
		Version: build.BuilderBuildKit,
		Tags:    []string{"prune-progress-build:latest"},
	})
	assert.NilError(t, err)
	defer built.Body.Close()
	buildMessages := json.NewDecoder(built.Body)
	for {
		var message jsonstream.Message
		err := buildMessages.Decode(&message)
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)
		assert.Assert(t, message.Error == nil, "build error: %v", message.Error)
	}

	for _, tc := range []struct {
		endpoint, reportField, expectedID string
		filters                           string
	}{
		{endpoint: "containers", reportField: "ContainersDeleted", expectedID: containerID},
		{endpoint: "networks", reportField: "NetworksDeleted", expectedID: "prune-progress-network"},
		{endpoint: "volumes", reportField: "VolumesDeleted", expectedID: "prune-progress-volume", filters: `{"all":["true"]}`},
		{endpoint: "images", reportField: "ImagesDeleted", filters: `{"dangling":["false"]}`},
		{endpoint: "build", reportField: "CachesDeleted"},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			query := url.Values{"stream": {"true"}}
			if tc.filters != "" {
				query.Set("filters", tc.filters)
			}
			if tc.endpoint == "build" {
				query.Set("all", "true")
			}
			resp, body, err := request.Post(ctx, "/v1.56/"+tc.endpoint+"/prune?"+query.Encode(), request.Host(d.Sock()))
			assert.NilError(t, err)
			defer body.Close()
			assert.Equal(t, resp.StatusCode, http.StatusOK)
			assert.Equal(t, resp.Header.Get("Content-Type"), "application/jsonl")
			dec := json.NewDecoder(body)
			var progress []jsonstream.Message
			var report map[string]json.RawMessage
			for {
				var message jsonstream.Message
				assert.NilError(t, dec.Decode(&message))
				assert.Assert(t, message.Error == nil, "prune error: %v", message.Error)
				if message.Aux != nil {
					assert.NilError(t, json.Unmarshal(*message.Aux, &report))
					break
				}
				progress = append(progress, message)
			}
			assert.ErrorIs(t, dec.Decode(new(any)), io.EOF)
			assert.Assert(t, report[tc.reportField] != nil)
			if tc.endpoint == "images" {
				var deleted []image.DeleteResponse
				assert.NilError(t, json.Unmarshal(report[tc.reportField], &deleted))
				assert.Assert(t, len(deleted) > 0)
				var expected []jsonstream.Message
				for _, entry := range deleted {
					if entry.Untagged != "" {
						expected = append(expected, jsonstream.Message{ID: entry.Untagged, Status: "untagged"})
					}
					if entry.Deleted != "" {
						expected = append(expected, jsonstream.Message{ID: entry.Deleted, Status: "deleted"})
					}
				}
				assert.DeepEqual(t, progress, expected)
				return
			}
			if tc.endpoint == "build" {
				var deleted []string
				assert.NilError(t, json.Unmarshal(report[tc.reportField], &deleted))
				assert.Assert(t, len(deleted) > 0, "expected populated build cache to be pruned")
				var expected []jsonstream.Message
				for _, id := range deleted {
					expected = append(expected, jsonstream.Message{ID: id, Status: "deleted"})
				}
				assert.DeepEqual(t, progress, expected)
				return
			}
			found := false
			for _, message := range progress {
				if message.ID == tc.expectedID {
					found = true
					assert.Equal(t, message.Status, "deleted")
				}
			}
			assert.Assert(t, found, "expected a progress message for %s", tc.expectedID)
			if tc.endpoint != "images" {
				var deleted []string
				assert.NilError(t, json.Unmarshal(report[tc.reportField], &deleted))
				assert.DeepEqual(t, deleted, []string{tc.expectedID})
			}
		})
	}
}
