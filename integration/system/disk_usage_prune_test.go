package system

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/image"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestDiskUsageConcurrentPrune tests that running DiskUsage concurrently with
// image removal does not cause an error.
// Regression test for https://github.com/moby/moby/issues/51978
func TestDiskUsageConcurrentPrune(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, !testEnv.UsingSnapshotter(), "only happens with containerd image store")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	// Load unique images with multiple layers each.
	// More snapshots = higher chance of hitting the race.
	const numImages = 10
	const layersPerImage = 10
	imageNames := make([]string, 0, numImages)
	for i := range numImages {
		var layers []specialimage.SingleFileLayer
		for j := range layersPerImage {
			layers = append(layers, specialimage.SingleFileLayer{
				Name:    fmt.Sprintf("file-%d-%d", i, j),
				Content: fmt.Appendf(nil, "content-%d-%d", i, j),
			})
		}
		imageName := fmt.Sprintf("test-image-%d:latest", i)
		imageNames = append(imageNames, imageName)
		image.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
			return specialimage.MultiLayerCustom(dir, imageName, layers)
		})
	}

	t.Cleanup(func() {
		for _, name := range imageNames {
			_, _ = apiClient.ImageRemove(ctx, name, client.ImageRemoveOptions{Force: true})
		}
	})

	const diskUsageCount = 5
	var wg sync.WaitGroup
	errCh := make(chan error, diskUsageCount)
	removeStarted := make(chan struct{})

	for range diskUsageCount {
		wg.Go(func() {
			<-removeStarted
			time.Sleep(time.Millisecond)
			_, err := apiClient.DiskUsage(ctx, client.DiskUsageOptions{
				Images: true,
			})
			if err != nil {
				errCh <- err
			}
		})
	}

	wg.Go(func() {
		close(removeStarted)
		for _, name := range imageNames {
			_, _ = apiClient.ImageRemove(ctx, name, client.ImageRemoveOptions{Force: true})
		}
	})

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NilError(t, err, "DiskUsage should not error when images are being removed concurrently")
	}
}
