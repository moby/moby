package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
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
		expectedURL     = "/images/image_id/history"
		historyResponse = `[{"Comment":"","Created":0,"CreatedBy":"","Id":"image_id1","Size":0,"Tags":["tag1","tag2"]},{"Comment":"","Created":0,"CreatedBy":"","Id":"image_id2","Size":0,"Tags":["tag1","tag2"]}]`
	)
	client := &Client{
		client: newMockClient(func(r *http.Request) (*http.Response, error) {
			assert.Check(t, is.Equal(r.URL.Path, expectedURL))
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

	imageHistories, err := client.ImageHistory(context.Background(), "image_id", image.HistoryOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(imageHistories, expected))
}
