package store

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	volumetestutils "github.com/docker/docker/volume/testutils"
	"github.com/stretchr/testify/require"
)

func TestRestore(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-restore")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	driverName := "test-restore"
	volumedrivers.Register(volumetestutils.NewFakeDriver(driverName), driverName)
	defer volumedrivers.Unregister("test-restore")

	s, err := New(dir)
	require.NoError(t, err)
	defer s.Shutdown()

	_, err = s.Create("test1", driverName, nil, nil)
	require.NoError(t, err)

	testLabels := map[string]string{"a": "1"}
	testOpts := map[string]string{"foo": "bar"}
	_, err = s.Create("test2", driverName, testOpts, testLabels)
	require.NoError(t, err)

	s.Shutdown()

	s, err = New(dir)
	require.NoError(t, err)

	v, err := s.Get("test1")
	require.NoError(t, err)

	dv := v.(volume.DetailedVolume)
	var nilMap map[string]string
	require.Equal(t, nilMap, dv.Options())
	require.Equal(t, nilMap, dv.Labels())

	v, err = s.Get("test2")
	require.NoError(t, err)
	dv = v.(volume.DetailedVolume)
	require.Equal(t, testOpts, dv.Options())
	require.Equal(t, testLabels, dv.Labels())
}
