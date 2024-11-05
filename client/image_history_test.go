package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageHistoryError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageHistory(context.Background(), "nothing", image.HistoryOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageHistory(t *testing.T) {
	const (
		expectedURL      = "/images/image_id/history"
		historyResponse  = `[{"Comment":"","Created":0,"CreatedBy":"","Id":"image_id1","Size":0,"Tags":["tag1","tag2"]},{"Comment":"","Created":0,"CreatedBy":"","Id":"image_id2","Size":0,"Tags":["tag1","tag2"]}]`
		expectedPlatform = `{"architecture":"arm64","os":"linux","variant":"v8"}`
	)
	client := &Client{
		client: newMockClient(func(r *http.Request) (*http.Response, error) {
			assert.Check(t, is.Equal(r.URL.Path, expectedURL))
			assert.Check(t, is.Equal(r.URL.Query().Get("platform"), expectedPlatform))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(historyResponse)),
			}, nil
		}),
	}
	expected := []image.HistoryResponseItem{
		{
			ID:   "image_id1",
			Tags: []string{"tag1", "tag2"},
		},
		{
			ID:   "image_id2",
			Tags: []string{"tag1", "tag2"},
		},
	}

	imageHistories, err := client.ImageHistory(context.Background(), "image_id", image.HistoryOptions{
		Platform: &ocispec.Platform{
			Architecture: "arm64",
			OS:           "linux",
			Variant:      "v8",
		},
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(imageHistories, expected))
}
