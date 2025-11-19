package image

import (
	"testing"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// Test case for 30027: image size reported as -1
// in v1.12 client against v1.13 daemon.
func TestImagesSizeCompatibility(t *testing.T) {
	ctx := setupTest(t)

	testCases := []struct {
		name       string
		apiVersion string
	}{
		{name: "LatestAPIVersion", apiVersion: ""},
		{name: "MinimumAPIVersion", apiVersion: client.MinAPIVersion},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli, err := client.New(
				client.FromEnv,
				client.WithAPIVersion(tc.apiVersion),
			)
			assert.NilError(t, err)
			defer cli.Close()

			images, err := cli.ImageList(ctx, client.ImageListOptions{})
			assert.NilError(t, err)
			assert.Assert(t, len(images.Items) > 0)

			for _, img := range images.Items {
				assert.Check(t, img.Size >= 0)
			}
		})
	}
}
