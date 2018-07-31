package daemon

import (
	"testing"
	"context"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/container"
	is "gotest.tools/assert/cmp"
	"github.com/docker/docker/daemon/images"
	"gotest.tools/assert"
)

func TestContainerPrune(t *testing.T) {
	ctx := context.Background()
	c := newContainerWithState(container.NewState())
	d, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()

	d.imageService = images.NewImageService(images.ImageServiceConfig{})
	d.imageService.GetContainerLayerSize = func(containerID string) (int64) { return 1 }
	d.containers.Add(c.ID, c)

	pruneReport, err := d.ContainersPrune(ctx, filters.NewArgs(filters.Arg("dryRun", "true")))
	assert.Assert(t, err)
	assert.Assert(t, is.Equal(pruneReport.ContainersDeleted[0], "container-1"))
}

func TestNetworksPrune(t *testing.T) {
	//service, cleanup := newTestService(t, ds)
	//defer cleanup()
	ctx := context.Background()

	daemon, cleanup := newDaemonWithTmpRoot(t)
	defer cleanup()

	daemon.NetworksPrune(ctx, filters.NewArgs(filters.Arg("label", "banana")))

	//_, err := service.Create(ctx, "dry-test", volume.DefaultDriverName)
	//assert.Assert(t, err)
	//
	//pr, err := service.Prune(ctx, filters.NewArgs(filters.Arg("dryRun", "true")))
	//assert.Assert(t, err)
	//assert.Assert(t, is.Len(pr.VolumesDeleted, 1))
	//assert.Assert(t, is.Equal(pr.VolumesDeleted[0], "dry-test"))
	//dv, err := service.Get(ctx, "dry-test")
	//assert.Assert(t, err)
	//assert.Assert(t, is.Equal(dv.Name, "dry-test"))
}
