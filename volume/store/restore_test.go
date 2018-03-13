package store

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	volumetestutils "github.com/docker/docker/volume/testutils"
	"github.com/gotestyourself/gotestyourself/assert"
)

func TestRestore(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-restore")
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	driverName := "test-restore"
	volumedrivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)
	defer volumedrivers.Unregister("test-restore")

	s, err := New(dir)
	assert.NilError(t, err)
	defer s.Shutdown()

	_, err = s.Create("test1", driverName, nil, nil)
	assert.NilError(t, err)

	testLabels := map[string]string{"a": "1"}
	testOpts := map[string]string{"foo": "bar"}
	_, err = s.Create("test2", driverName, testOpts, testLabels)
	assert.NilError(t, err)

	s.Shutdown()

	s, err = New(dir)
	assert.NilError(t, err)

	v, err := s.Get("test1")
	assert.NilError(t, err)

	dv := v.(volume.DetailedVolume)
	var nilMap map[string]string
	assert.DeepEqual(t, nilMap, dv.Options())
	assert.DeepEqual(t, nilMap, dv.Labels())

	v, err = s.Get("test2")
	assert.NilError(t, err)
	dv = v.(volume.DetailedVolume)
	assert.DeepEqual(t, testOpts, dv.Options())
	assert.DeepEqual(t, testLabels, dv.Labels())
}
