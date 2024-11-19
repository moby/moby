package container // import "github.com/docker/docker/integration/container"

import (
	"runtime"
	"strings"
	"testing"

	"github.com/containerd/platforms"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestInspectAnnotations(t *testing.T) {
	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t)

	annotations := map[string]string{
		"hello": "world",
		"foo":   "bar",
	}

	name := strings.ToLower(t.Name())
	id := container.Create(ctx, t, apiClient,
		container.WithName(name),
		container.WithCmd("true"),
		func(c *container.TestContainerConfig) {
			c.HostConfig.Annotations = annotations
		},
	)

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(inspect.HostConfig.Annotations, annotations))
}

// TestNetworkAliasesAreEmpty verifies that network-scoped aliases are not set
// for non-custom networks (network-scoped aliases are only supported for
// custom networks, except for the "Default Switch" network on Windows).
func TestNetworkAliasesAreEmpty(t *testing.T) {
	ctx := setupTest(t)
	apiClient := request.NewAPIClient(t)

	netModes := []string{"host", "bridge", "none"}
	if runtime.GOOS == "windows" {
		netModes = []string{"nat", "none"}
	}

	for _, nwMode := range netModes {
		t.Run(nwMode, func(t *testing.T) {
			ctr := container.Create(ctx, t, apiClient,
				container.WithName("ctr-"+nwMode),
				container.WithImage("busybox:latest"),
				container.WithNetworkMode(nwMode))
			defer apiClient.ContainerRemove(ctx, ctr, containertypes.RemoveOptions{
				Force: true,
			})

			inspect := container.Inspect(ctx, t, apiClient, ctr)
			netAliases := inspect.NetworkSettings.Networks[nwMode].Aliases

			assert.Check(t, is.Nil(netAliases))
		})
	}
}

func TestInspectImageManifestPlatform(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !testEnv.UsingSnapshotter())

	tests := []struct {
		name             string
		image            string
		skipIf           func() bool
		expectedPlatform platforms.Platform
	}{
		{
			name:  "amd64 only on any host",
			image: "busybox:latest",
			expectedPlatform: platforms.Platform{
				OS:           "linux",
				Architecture: "amd64",
				Variant:      "",
			},
		},
		{
			skipIf: func() bool { return runtime.GOARCH != "amd64" },
			name:   "amd64 image on non-amd64 host",

			image: "hello-world:amd64",
			expectedPlatform: platforms.Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
		},
		{
			name:   "arm64 image on non-arm64 host",
			skipIf: func() bool { return runtime.GOARCH != "arm64" },
			image:  "hello-world:arm64",

			expectedPlatform: platforms.Platform{
				OS:           "linux",
				Architecture: "arm64",
				Variant:      "",
			},
		},
	}

	for _, tc := range tests {
		if tc.skipIf != nil && tc.skipIf() {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTest(t)
			apiClient := request.NewAPIClient(t)

			ctr := container.Create(ctx, t, apiClient, container.WithImage(tc.image))
			defer apiClient.ContainerRemove(ctx, ctr, containertypes.RemoveOptions{Force: true})

			img, _, err := apiClient.ImageInspectWithRaw(ctx, tc.image)
			assert.NilError(t, err)

			hostPlatform := platforms.Platform{
				OS:           img.Os,
				Architecture: img.Architecture,
				Variant:      img.Variant,
			}
			inspect := container.Inspect(ctx, t, apiClient, ctr)
			assert.Assert(t, inspect.ImageManifestDescriptor != nil)
			assert.Check(t, is.DeepEqual(*inspect.ImageManifestDescriptor.Platform, hostPlatform))

			t.Run("pre 1.48", func(t *testing.T) {
				oldClient := request.NewAPIClient(t, client.WithVersion("1.47"))
				inspect := container.Inspect(ctx, t, oldClient, ctr)
				assert.Check(t, is.Nil(inspect.ImageManifestDescriptor))
			})
		})
	}
}
