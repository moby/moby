package graphdriver

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsEmptyDir(t *testing.T) {
	tmp, err := ioutil.TempDir("", "test-is-empty-dir")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	d := filepath.Join(tmp, "empty-dir")
	err = os.Mkdir(d, 0755)
	require.NoError(t, err)
	empty := isEmptyDir(d)
	assert.True(t, empty)

	d = filepath.Join(tmp, "dir-with-subdir")
	err = os.MkdirAll(filepath.Join(d, "subdir"), 0755)
	require.NoError(t, err)
	empty = isEmptyDir(d)
	assert.False(t, empty)

	d = filepath.Join(tmp, "dir-with-empty-file")
	err = os.Mkdir(d, 0755)
	require.NoError(t, err)
	_, err = ioutil.TempFile(d, "file")
	require.NoError(t, err)
	empty = isEmptyDir(d)
	assert.False(t, empty)
}
