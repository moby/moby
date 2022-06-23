package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDistributionInspectUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.29"),
	)
	assert.NilError(t, err)
	_, err = client.DistributionInspect(context.Background(), "foobar:1.0", "")
	assert.Check(t, is.Error(err, `"distribution inspect" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestDistributionInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, err = client.DistributionInspect(context.Background(), "", "")
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}
