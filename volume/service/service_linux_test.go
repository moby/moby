package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/docker/docker/volume/service/opts"
	"github.com/docker/docker/volume/testutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLocalVolumeSize(t *testing.T) {
	t.Parallel()

	ds := volumedrivers.NewStore(nil)
	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	l, err := local.New(dir, idtools.Identity{UID: os.Getuid(), GID: os.Getegid()})
	assert.NilError(t, err)
	assert.Assert(t, ds.Register(l, volume.DefaultDriverName))
	assert.Assert(t, ds.Register(testutils.NewFakeDriver("fake"), "fake"))

	service, cleanup := newTestService(t, ds)
	defer cleanup()

	ctx := context.Background()
	v1, err := service.Create(ctx, "test1", volume.DefaultDriverName, opts.WithCreateReference("foo"))
	assert.NilError(t, err)
	v2, err := service.Create(ctx, "test2", volume.DefaultDriverName)
	assert.NilError(t, err)
	_, err = service.Create(ctx, "test3", "fake")
	assert.NilError(t, err)

	data := make([]byte, 1024)
	err = os.WriteFile(filepath.Join(v1.Mountpoint, "data"), data, 0o644)
	assert.NilError(t, err)
	err = os.WriteFile(filepath.Join(v2.Mountpoint, "data"), data[:1], 0o644)
	assert.NilError(t, err)

	ls, err := service.LocalVolumesSize(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(ls, 2))

	for _, v := range ls {
		switch v.Name {
		case "test1":
			assert.Assert(t, is.Equal(v.UsageData.Size, int64(len(data))))
			assert.Assert(t, is.Equal(v.UsageData.RefCount, int64(1)))
		case "test2":
			assert.Assert(t, is.Equal(v.UsageData.Size, int64(len(data[:1]))))
			assert.Assert(t, is.Equal(v.UsageData.RefCount, int64(0)))
		default:
			t.Fatalf("got unexpected volume: %+v", v)
		}
	}
}
