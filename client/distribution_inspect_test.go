package client // import "github.com/docker/docker/client"

import (
	"context"
	"net/http"
	"testing"

	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
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
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}
