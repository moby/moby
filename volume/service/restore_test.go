package service // import "github.com/docker/docker/volume/service"

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/service/opts"
	volumetestutils "github.com/docker/docker/volume/testutils"
	"github.com/gotestyourself/gotestyourself/assert"
)

func TestRestore(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-restore")
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	drivers := volumedrivers.NewStore(nil)
	driverName := "test-restore"
	drivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)

	s, err := NewStore(dir, drivers)
	assert.NilError(t, err)
	defer s.Shutdown()

	ctx := context.Background()
	_, err = s.Create(ctx, "test1", driverName)
	assert.NilError(t, err)

	testLabels := map[string]string{"a": "1"}
	testOpts := map[string]string{"foo": "bar"}
	_, err = s.Create(ctx, "test2", driverName, opts.WithCreateOptions(testOpts), opts.WithCreateLabels(testLabels))
	assert.NilError(t, err)

	s.Shutdown()

	s, err = NewStore(dir, drivers)
	assert.NilError(t, err)

	v, err := s.Get(ctx, "test1")
	assert.NilError(t, err)

	dv := v.(volume.DetailedVolume)
	var nilMap map[string]string
	assert.DeepEqual(t, nilMap, dv.Options())
	assert.DeepEqual(t, nilMap, dv.Labels())

	v, err = s.Get(ctx, "test2")
	assert.NilError(t, err)
	dv = v.(volume.DetailedVolume)
	assert.DeepEqual(t, testOpts, dv.Options())
	assert.DeepEqual(t, testLabels, dv.Labels())
}
