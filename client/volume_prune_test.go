package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumePruneError(t *testing.T) {
	client := &Client{
		version: "1.42",
		client:  &http.Client{},
	}

	_, err := client.VolumesPrune(context.Background(), volume.PruneOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("all", "true")),
	})
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.Error(err, `conflicting options: cannot specify both "all"" and "all" filter"`))
}
