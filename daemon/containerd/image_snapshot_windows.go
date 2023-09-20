package containerd

import (
	"context"

	"github.com/containerd/containerd/snapshots"
)

func (i *ImageService) remapSnapshot(ctx context.Context, snapshotter snapshots.Snapshotter, id string, parentSnapshot string) error {
	return nil
}
