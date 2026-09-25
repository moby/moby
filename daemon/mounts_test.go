package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/volume/mounts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPrepareMountPointsSerializesLiveRestoreWithCleanup(t *testing.T) {
	volume := &blockingLiveRestoreVolume{
		restoreStarted:  make(chan struct{}),
		continueRestore: make(chan struct{}),
	}
	mountPoint := &mounts.MountPoint{
		Volume: volume,
		ID:     "mount-id",
	}
	ctr := container.NewBaseContainer("container-id", t.TempDir())
	ctr.State.Running = true
	ctr.MountPoints["/volume"] = mountPoint

	restoreErr := make(chan error, 1)
	go func() {
		restoreErr <- (&Daemon{}).prepareMountPoints(ctr)
	}()
	<-volume.restoreStarted

	cleanupBeforeRestore := ctr.TryLock()
	var cleanupErr error
	if cleanupBeforeRestore {
		cleanupErr = mountPoint.Cleanup(context.Background())
		ctr.Unlock()
	}

	close(volume.continueRestore)
	assert.NilError(t, <-restoreErr)

	if !cleanupBeforeRestore {
		ctr.Lock()
		cleanupErr = mountPoint.Cleanup(context.Background())
		ctr.Unlock()
	}
	assert.NilError(t, cleanupErr)
	assert.Check(t, is.Equal(volume.active.Load(), int64(0)))
	assert.Check(t, is.Equal(volume.unmounts.Load(), int64(1)))
}

type blockingLiveRestoreVolume struct {
	active          atomic.Int64
	unmounts        atomic.Int64
	restoreStarted  chan struct{}
	continueRestore chan struct{}
}

func (v *blockingLiveRestoreVolume) Name() string {
	return "test-volume"
}

func (v *blockingLiveRestoreVolume) DriverName() string {
	return "test"
}

func (v *blockingLiveRestoreVolume) Path() string {
	return ""
}

func (v *blockingLiveRestoreVolume) Mount(string) (string, error) {
	return "", nil
}

func (v *blockingLiveRestoreVolume) Unmount(string) error {
	v.active.Add(-1)
	v.unmounts.Add(1)
	return nil
}

func (v *blockingLiveRestoreVolume) CreatedAt() (time.Time, error) {
	return time.Time{}, nil
}

func (v *blockingLiveRestoreVolume) Status() map[string]any {
	return nil
}

func (v *blockingLiveRestoreVolume) LiveRestoreVolume(context.Context, string) error {
	close(v.restoreStarted)
	<-v.continueRestore
	v.active.Add(1)
	return nil
}
