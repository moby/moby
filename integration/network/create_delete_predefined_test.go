package network

import (
	"slices"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCreateDeletePredefinedNetworks(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Predefined networks differ per OS.
	predefined := []string{"bridge", "host", "none"}
	if testEnv.DaemonInfo.OSType == "windows" {
		predefined = []string{"nat", "none"}
	}

	// Verify the daemon actually has those networks.
	nws, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	assert.NilError(t, err)

	var actual []string
	for _, nw := range nws {
		if slices.Contains(predefined, nw.Name) {
			actual = append(actual, nw.Name)
		}
	}
	slices.Sort(actual)
	slices.Sort(predefined)
	assert.Check(t, is.DeepEqual(actual, predefined))

	for _, name := range predefined {
		t.Run(name, func(t *testing.T) {
			// Creating a predefined network must fail.
			_, err := apiClient.NetworkCreate(ctx, name, client.NetworkCreateOptions{})
			assert.Check(t, is.ErrorContains(err, "pre-defined"))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))

			// Deleting a predefined network must fail.
			err = apiClient.NetworkRemove(ctx, name)
			assert.Check(t, is.ErrorContains(err, "pre-defined"))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))

			// Sanity: it should still exist.
			_, err = apiClient.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
			assert.NilError(t, err)
		})
	}
}
