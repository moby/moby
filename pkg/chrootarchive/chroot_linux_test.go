package chrootarchive

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChroot(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "docker-TestChroot1")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destination := filepath.Join(tempDir, "dest")

	err = system.MkdirAll(destination, 0700)
	require.NoError(t, err)

	err = chroot("")
	assert.Error(t, err, "Error after fallback to chroot: no such file or directory")

	err = chroot(destination)
	assert.NoError(t, err)
}
