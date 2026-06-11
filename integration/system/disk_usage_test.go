package system

import (
	"net/netip"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	containertypes "github.com/moby/moby/api/types/container"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDiskUsage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows") // d.Start fails on Windows with `protocol not available`

	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Cleanup(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	var stepDU client.DiskUsageResult
	for _, step := range []struct {
		doc  string
		next func(t *testing.T, prev client.DiskUsageResult) client.DiskUsageResult
	}{
		{
			doc: "empty",
			next: func(t *testing.T, _ client.DiskUsageResult) client.DiskUsageResult {
				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{
					Images:     true,
					Containers: true,
					BuildCache: true,
					Volumes:    true,
					Verbose:    true,
				})
				assert.NilError(t, err)

				expectedLayersSize := int64(0)
				// TODO: Investigate https://github.com/moby/moby/issues/47119
				// Make 4096 (block size) also a valid value for zero usage.
				if testEnv.UsingSnapshotter() && testEnv.IsRootless() {
					if du.Images.TotalSize == 4096 {
						expectedLayersSize = 4096
					}
				}

				assert.DeepEqual(t, du, client.DiskUsageResult{
					Containers: client.ContainersDiskUsage{},
					Images: client.ImagesDiskUsage{
						TotalSize: expectedLayersSize,
					},
					BuildCache: client.BuildCacheDiskUsage{},
					Volumes:    client.VolumesDiskUsage{},
				})
				return du
			},
		},
		{
			doc: "after LoadBusybox",
			next: func(t *testing.T, _ client.DiskUsageResult) client.DiskUsageResult {
				d.LoadBusybox(ctx, t)

				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{
					Images:     true,
					Containers: true,
					BuildCache: true,
					Volumes:    true,
					Verbose:    true,
				})
				assert.NilError(t, err)

				assert.Equal(t, du.Images.ActiveCount, int64(0))
				assert.Equal(t, du.Images.TotalCount, int64(1))
				assert.Equal(t, du.Images.Reclaimable, du.Images.TotalSize)
				assert.Assert(t, du.Images.TotalSize > 0)
				assert.Equal(t, len(du.Images.Items), 1)
				assert.Equal(t, len(du.Images.Items[0].RepoTags), 1)
				assert.Check(t, is.Equal(du.Images.Items[0].RepoTags[0], "busybox:latest"))

				// Image size is layer size + content size. Content size is included in layers size.
				assert.Equal(t, du.Images.Items[0].Size, du.Images.TotalSize)

				return du
			},
		},
		{
			doc: "after container.Run",
			next: func(t *testing.T, prev client.DiskUsageResult) client.DiskUsageResult {
				cID := container.Run(ctx, t, apiClient)

				du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{
					Images:     true,
					Containers: true,
					BuildCache: true,
					Volumes:    true,
					Verbose:    true,
				})
				assert.NilError(t, err)

				assert.Equal(t, du.Containers.ActiveCount, int64(1))
				assert.Equal(t, du.Containers.TotalCount, int64(1))
				assert.Equal(t, len(du.Containers.Items), 1)
				assert.Equal(t, len(du.Containers.Items[0].Names), 1)
				assert.Assert(t, len(prev.Images.Items) > 0)
				assert.Check(t, du.Containers.Items[0].Created >= prev.Images.Items[0].Created)

				// Additional container layer could add to the size
				assert.Check(t, du.Images.TotalSize >= prev.Images.TotalSize)

				assert.Equal(t, du.Images.ActiveCount, int64(1))
				assert.Equal(t, du.Images.TotalCount, int64(1))
				assert.Equal(t, du.Images.Reclaimable, int64(0))
				assert.Equal(t, len(du.Images.Items), 1)
				assert.Equal(t, du.Images.Items[0].Containers, prev.Images.Items[0].Containers+1)

				assert.Check(t, is.Equal(du.Containers.Items[0].ID, cID))
				assert.Check(t, is.Equal(du.Containers.Items[0].Image, "busybox"))
				assert.Check(t, is.Equal(du.Containers.Items[0].ImageID, prev.Images.Items[0].ID))

				// ImageManifestDescriptor should NOT be populated.
				assert.Check(t, is.Nil(du.Containers.Items[0].ImageManifestDescriptor))

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
				expected client.DiskUsageResult
			}{
				{
					doc: "container types",
					options: client.DiskUsageOptions{
						Containers: true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: stepDU.Containers,
						Images:     client.ImagesDiskUsage{},
						BuildCache: client.BuildCacheDiskUsage{},
						Volumes:    client.VolumesDiskUsage{},
					},
				},
				{
					doc: "image types",
					options: client.DiskUsageOptions{
						Images:  true,
						Verbose: true,
					},
					expected: client.DiskUsageResult{
						Containers: client.ContainersDiskUsage{},
						Images:     stepDU.Images,
						BuildCache: client.BuildCacheDiskUsage{},
						Volumes:    client.VolumesDiskUsage{},
					},
				},
				{
					doc: "volume types",
					options: client.DiskUsageOptions{
						Volumes: true,
						Verbose: true,
					},
					expected: client.DiskUsageResult{
						Containers: client.ContainersDiskUsage{},
						Images:     client.ImagesDiskUsage{},
						BuildCache: client.BuildCacheDiskUsage{},
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "build-cache types",
					options: client.DiskUsageOptions{
						BuildCache: true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: client.ContainersDiskUsage{},
						Images:     client.ImagesDiskUsage{},
						BuildCache: stepDU.BuildCache,
						Volumes:    client.VolumesDiskUsage{},
					},
				},
				{
					doc: "container, volume types",
					options: client.DiskUsageOptions{
						Containers: true,
						Volumes:    true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: stepDU.Containers,
						Images:     client.ImagesDiskUsage{},
						BuildCache: client.BuildCacheDiskUsage{},
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "image, build-cache types",
					options: client.DiskUsageOptions{
						Images:     true,
						BuildCache: true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: client.ContainersDiskUsage{},
						Images:     stepDU.Images,
						BuildCache: stepDU.BuildCache,
						Volumes:    client.VolumesDiskUsage{},
					},
				},
				{
					doc: "container, volume, build-cache types",
					options: client.DiskUsageOptions{
						Containers: true,
						BuildCache: true,
						Volumes:    true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: stepDU.Containers,
						Images:     client.ImagesDiskUsage{},
						BuildCache: stepDU.BuildCache,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "image, volume, build-cache types",
					options: client.DiskUsageOptions{
						Images:     true,
						BuildCache: true,
						Volumes:    true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: client.ContainersDiskUsage{},
						Images:     stepDU.Images,
						BuildCache: stepDU.BuildCache,
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "container, image, volume types",
					options: client.DiskUsageOptions{
						Containers: true,
						Images:     true,
						Volumes:    true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						BuildCache: client.BuildCacheDiskUsage{},
						Volumes:    stepDU.Volumes,
					},
				},
				{
					doc: "container, image, volume, build-cache types",
					options: client.DiskUsageOptions{
						Containers: true,
						Images:     true,
						BuildCache: true,
						Volumes:    true,
						Verbose:    true,
					},
					expected: client.DiskUsageResult{
						Containers: stepDU.Containers,
						Images:     stepDU.Images,
						BuildCache: stepDU.BuildCache,
						Volumes:    stepDU.Volumes,
					},
				},
			} {
				t.Run(tc.doc, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					// TODO: Run in parallel once https://github.com/moby/moby/pull/42560 is merged.

					du, err := apiClient.DiskUsage(ctx, tc.options)
					assert.NilError(t, err)
					assert.DeepEqual(t, du, tc.expected,
						cmpopts.EquateComparable(netip.Addr{}, netip.Prefix{}),
						cmpopts.IgnoreFields(containertypes.Summary{}, "Status"),
					)
				})
			}
		})
	}
}

func TestDiskUsageImageSharedUsage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows") // d.Start fails on Windows with `protocol not available`

	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Cleanup(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)
	apiClient := d.NewClientT(t)

	d.LoadBusybox(ctx, t)

	imageRefs := []string{
		"shared-usage-a:latest",
		"shared-usage-b:latest",
	}
	for _, imageRef := range imageRefs {
		build.Do(ctx, t, apiClient,
			fakecontext.New(t, "", fakecontext.WithDockerfile("FROM busybox\nRUN echo "+imageRef+" > /shared-usage\n")),
			client.ImageBuildOptions{Tags: []string{imageRef}},
		)
	}

	du, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{
		Images:  true,
		Verbose: true,
	})
	assert.NilError(t, err)

	summariesByTag := imageSummariesByTag(du.Images.Items)
	for _, imageRef := range imageRefs {
		summary, ok := summariesByTag[imageRef]
		assert.Assert(t, ok, "expected disk usage output to include %s", imageRef)
		assert.Check(t, summary.SharedSize > 0, "expected %s to report shared image size, got %d", imageRef, summary.SharedSize)
		assert.Check(t, summary.Size > summary.SharedSize, "expected %s to have unique image size, got size=%d shared=%d", imageRef, summary.Size, summary.SharedSize)
	}
}

func imageSummariesByTag(items []imagetypes.Summary) map[string]imagetypes.Summary {
	summariesByTag := make(map[string]imagetypes.Summary)
	for _, item := range items {
		for _, tag := range item.RepoTags {
			summariesByTag[tag] = item
		}
	}
	return summariesByTag
}
