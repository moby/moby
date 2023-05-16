package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestDiskUsage(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows") // d.Start fails on Windows with `protocol not available`

	t.Parallel()

	d := daemon.New(t)
	defer d.Cleanup(t)
	d.Start(t, "--iptables=false")
	defer d.Stop(t)
	client := d.NewClientT(t)

	ctx := context.Background()

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
				assert.DeepEqual(t, du, types.DiskUsage{
					Images:     []*types.ImageSummary{},
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
				d.LoadBusybox(t)

				du, err := client.DiskUsage(ctx, types.DiskUsageOptions{})
				assert.NilError(t, err)
				assert.Assert(t, du.LayersSize > 0)
				assert.Equal(t, len(du.Images), 1)
				assert.DeepEqual(t, du, types.DiskUsage{
					LayersSize: du.LayersSize,
					Images: []*types.ImageSummary{
						{
							Created:  du.Images[0].Created,
							ID:       du.Images[0].ID,
							RepoTags: []string{"busybox:latest"},
							Size:     du.LayersSize,
						},
					},
					Containers: []*types.Container{},
					Volumes:    []*volume.Volume{},
					BuildCache: []*types.BuildCache{},
				})
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
				assert.Assert(t, du.Containers[0].Created >= prev.Images[0].Created)
				assert.DeepEqual(t, du, types.DiskUsage{
					LayersSize: prev.LayersSize,
					Images: []*types.ImageSummary{
						func() *types.ImageSummary {
							sum := *prev.Images[0]
							sum.Containers++
							return &sum
						}(),
					},
					Containers: []*types.Container{
						{
							ID:              cID,
							Names:           du.Containers[0].Names,
							Image:           "busybox",
							ImageID:         prev.Images[0].ID,
							Command:         du.Containers[0].Command, // not relevant for the test
							Created:         du.Containers[0].Created,
							Ports:           du.Containers[0].Ports, // not relevant for the test
							SizeRootFs:      prev.Images[0].Size,
							Labels:          du.Containers[0].Labels,          // not relevant for the test
							State:           du.Containers[0].State,           // not relevant for the test
							Status:          du.Containers[0].Status,          // not relevant for the test
							HostConfig:      du.Containers[0].HostConfig,      // not relevant for the test
							NetworkSettings: du.Containers[0].NetworkSettings, // not relevant for the test
							Mounts:          du.Containers[0].Mounts,          // not relevant for the test
						},
					},
					Volumes:    []*volume.Volume{},
					BuildCache: []*types.BuildCache{},
				})
				return du
			},
		},
	} {
		t.Run(step.doc, func(t *testing.T) {
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
					// TODO: Run in parallel once https://github.com/moby/moby/pull/42560 is merged.

					du, err := client.DiskUsage(ctx, tc.options)
					assert.NilError(t, err)
					assert.DeepEqual(t, du, tc.expected)
				})
			}
		})
	}
}
