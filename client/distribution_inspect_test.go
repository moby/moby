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
	client := &Client{
		version: "1.29",
		client:  &http.Client{},
	}
	_, err := client.DistributionInspect(context.Background(), "foobar:1.0", "")
	assert.Check(t, is.Error(err, `"distribution inspect" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestDistributionInspectWithEmptyID(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	}
	_, err := client.DistributionInspect(context.Background(), "", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}
