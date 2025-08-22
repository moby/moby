package system

import (
	"testing"

	"github.com/moby/moby/api/types/build"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDiskUsage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows") // d.Start fails on Windows with `protocol not available`

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Cleanup(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	var stepDU system.DiskUsage
	for _, step := range []struct {
		doc  string
		next func(t *testing.T, prev system.DiskUsage) system.DiskUsage
	}{
		{
			doc: "empty",
			next: func(t *testing.T, _ system.DiskUsage) system.DiskUsage {
				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{})
				assert.NilError(t, err)

				expectedLayersSize := int64(0)
				// TODO: Investigate https://github.com/moby/moby/issues/47119
				// Make 4096 (block size) also a valid value for zero usage.
				if testEnv.UsingSnapshotter() && testEnv.IsRootless() {
					if du.LayersSize == 4096 {
						expectedLayersSize = du.LayersSize
					}
				}

				assert.DeepEqual(t, du, system.DiskUsage{
					LayersSize: expectedLayersSize,
					Images:     []*image.Summary{},
					Containers: []*containertypes.Summary{},
					Volumes:    []*volume.Volume{},
					BuildCache: []*build.CacheRecord{},
				})
				return du
			},
		},
		{
			doc: "after LoadBusybox",
			next: func(t *testing.T, _ system.DiskUsage) system.DiskUsage {
				d.LoadBusybox(ctx, t)

				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{})
				assert.NilError(t, err)
				assert.Assert(t, du.LayersSize > 0)
				assert.Equal(t, len(du.Images), 1)
				assert.Equal(t, len(du.Images[0].RepoTags), 1)
				assert.Check(t, is.Equal(du.Images[0].RepoTags[0], "busybox:latest"))

				// Image size is layer size + content size. Content size is included in layers size.
				assert.Equal(t, du.Images[0].Size, du.LayersSize)

				return du
			},
		},
		{
			doc: "after container.Run",
			next: func(t *testing.T, prev system.DiskUsage) system.DiskUsage {
				cID := container.Run(ctx, t, apiClient)

				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{})
				assert.NilError(t, err)
				assert.Equal(t, len(du.Containers), 1)
				assert.Equal(t, len(du.Containers[0].Names), 1)
				assert.Assert(t, len(prev.Images) > 0)
				assert.Check(t, du.Containers[0].Created >= prev.Images[0].Created)

				// Additional container layer could add to the size
				assert.Check(t, du.LayersSize >= prev.LayersSize)

				assert.Equal(t, len(du.Images), 1)
				assert.Equal(t, du.Images[0].Containers, prev.Images[0].Containers+1)

				assert.Check(t, is.Equal(du.Containers[0].ID, cID))
				assert.Check(t, is.Equal(du.Containers[0].Image, "busybox"))
				assert.Check(t, is.Equal(du.Containers[0].ImageID, prev.Images[0].ID))

				// ImageManifestDescriptor should NOT be populated.
				assert.Check(t, is.Nil(du.Containers[0].ImageManifestDescriptor))

				return du
			},
		},
	} {
		t.Run(step.doc, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			stepDU = step.next(t, stepDU)

			for _, tc := range []struct {
				doc      string
				options  client.DiskUsageOptions
				expected system.DiskUsage
			}{
				{
					doc: "container types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ContainerObject,
						},
					},
					expected: system.DiskUsage{
						Containers: stepDU.Containers,
					},
				},
				{
					doc: "image types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ImageObject,
						},
					},
					expected: system.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
					},
				},
				{
					doc: "volume types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.VolumeObject,
						},
					},
					expected: system.DiskUsage{
						Volumes: stepDU.Volumes,
					},
				},
				{
					doc: "build-cache types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.BuildCacheObject,
						},
					},
					expected: system.DiskUsage{
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, volume types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ContainerObject,
							system.VolumeObject,
						},
					},
					expected: system.DiskUsage{
						Containers: stepDU.Containers,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "image, build-cache types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ImageObject,
							system.BuildCacheObject,
						},
					},
					expected: system.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, volume, build-cache types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ContainerObject,
							system.VolumeObject,
							system.BuildCacheObject,
						},
					},
					expected: system.DiskUsage{
						Containers: stepDU.Containers,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "image, volume, build-cache types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ImageObject,
							system.VolumeObject,
							system.BuildCacheObject,
						},
					},
					expected: system.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, image, volume types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ContainerObject,
							system.ImageObject,
							system.VolumeObject,
						},
					},
					expected: system.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "container, image, volume, build-cache types",
					options: client.DiskUsageOptions{
						Types: []system.DiskUsageObject{
							system.ContainerObject,
							system.ImageObject,
							system.VolumeObject,
							system.BuildCacheObject,
						},
					},
					expected: system.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
			} {
				t.Run(tc.doc, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					// TODO: Run in parallel once https://github.com/moby/moby/pull/42560 is merged.

					du, err := apiClient.DiskUsage(ctx, tc.options)
					assert.NilError(t, err)
					assert.DeepEqual(t, du, tc.expected)
				})
			}
		})
	}
}
