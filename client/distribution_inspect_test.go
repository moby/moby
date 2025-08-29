package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDistributionInspectUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(WithVersion("1.29"), WithHTTPClient(&http.Client{}))
	assert.NilError(t, err)
	_, err = client.DistributionInspect(context.Background(), "foobar:1.0", "")
	assert.Check(t, is.Error(err, `"distribution inspect" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestDistributionInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.DistributionInspect(context.Background(), "", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}
