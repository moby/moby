package system // import "github.com/docker/docker/integration/system"

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
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
	client := d.NewClientT(t)

	var stepDU types.DiskUsage
	for _, step := range []struct {
		doc  string
		next func(t *testing.T, prev types.DiskUsage) types.DiskUsage
	}{
		{
			doc: "empty",
			next: func(t *testing.T, _ types.DiskUsage) types.DiskUsage {
				du, err := client.DiskUsage(ctx, types.DiskUsageOptions{})
				assert.NilError(t, err)

				expectedLayersSize := int64(0)
				// TODO: Investigate https://github.com/moby/moby/issues/47119
				// Make 4096 (block size) also a valid value for zero usage.
				if testEnv.UsingSnapshotter() && testEnv.IsRootless() {
					if du.LayersSize == 4096 {
						expectedLayersSize = du.LayersSize
					}
				}

				assert.DeepEqual(t, du, types.DiskUsage{
					LayersSize: expectedLayersSize,
					Images:     []*image.Summary{},
					Containers: []*types.Container{},
					Volumes:    []*volume.Volume{},
					BuildCache: []*types.BuildCache{},
				})
				return du
			},
		},
		{
			doc: "after LoadBusybox",
			next: func(t *testing.T, _ types.DiskUsage) types.DiskUsage {
				d.LoadBusybox(ctx, t)

				du, err := client.DiskUsage(ctx, types.DiskUsageOptions{})
				assert.NilError(t, err)
				assert.Assert(t, du.LayersSize > 0)
				assert.Equal(t, len(du.Images), 1)
				assert.Equal(t, len(du.Images[0].RepoTags), 1)
				assert.Check(t, is.Equal(du.Images[0].RepoTags[0], "busybox:latest"))

				// Image size is layer size + content size, should be greater than total layer size
				assert.Assert(t, du.Images[0].Size >= du.LayersSize)

				// If size is greater, than content exists and should have a repodigest
				if du.Images[0].Size > du.LayersSize {
					assert.Equal(t, len(du.Images[0].RepoDigests), 1)
					assert.Check(t, strings.HasPrefix(du.Images[0].RepoDigests[0], "busybox@"))
				}

				return du
			},
		},
		{
			doc: "after container.Run",
			next: func(t *testing.T, prev types.DiskUsage) types.DiskUsage {
				cID := container.Run(ctx, t, client)

				du, err := client.DiskUsage(ctx, types.DiskUsageOptions{})
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

				// The rootfs size should be equivalent to all the layers,
				// previously used prev.Images[0].Size, which may differ from content data
				assert.Check(t, is.Equal(du.Containers[0].SizeRootFs, du.LayersSize))

				return du
			},
		},
	} {
		t.Run(step.doc, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			stepDU = step.next(t, stepDU)

			for _, tc := range []struct {
				doc      string
				options  types.DiskUsageOptions
				expected types.DiskUsage
			}{
				{
					doc: "container types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ContainerObject,
						},
					},
					expected: types.DiskUsage{
						Containers: stepDU.Containers,
					},
				},
				{
					doc: "image types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ImageObject,
						},
					},
					expected: types.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
					},
				},
				{
					doc: "volume types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.VolumeObject,
						},
					},
					expected: types.DiskUsage{
						Volumes: stepDU.Volumes,
					},
				},
				{
					doc: "build-cache types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.BuildCacheObject,
						},
					},
					expected: types.DiskUsage{
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, volume types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ContainerObject,
							types.VolumeObject,
						},
					},
					expected: types.DiskUsage{
						Containers: stepDU.Containers,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "image, build-cache types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ImageObject,
							types.BuildCacheObject,
						},
					},
					expected: types.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, volume, build-cache types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ContainerObject,
							types.VolumeObject,
							types.BuildCacheObject,
						},
					},
					expected: types.DiskUsage{
						Containers: stepDU.Containers,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "image, volume, build-cache types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ImageObject,
							types.VolumeObject,
							types.BuildCacheObject,
						},
					},
					expected: types.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
				{
					doc: "container, image, volume types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ContainerObject,
							types.ImageObject,
							types.VolumeObject,
						},
					},
					expected: types.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "container, image, volume, build-cache types",
					options: types.DiskUsageOptions{
						Types: []types.DiskUsageObject{
							types.ContainerObject,
							types.ImageObject,
							types.VolumeObject,
							types.BuildCacheObject,
						},
					},
					expected: types.DiskUsage{
						LayersSize: stepDU.LayersSize,
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						Volumes:    stepDU.Volumes,
						BuildCache: stepDU.BuildCache,
					},
				},
			} {
				tc := tc
				t.Run(tc.doc, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					// TODO: Run in parallel once https://github.com/moby/moby/pull/42560 is merged.

					du, err := client.DiskUsage(ctx, tc.options)
					assert.NilError(t, err)
					assert.DeepEqual(t, du, tc.expected)
				})
			}
		})
	}
}
