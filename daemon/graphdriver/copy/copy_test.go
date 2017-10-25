// +build linux

package copy

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCopyFileRangeSyscallAvailable(t *testing.T) {
	// Verifies:
	// 1. That copyFileRangeEnabled is being set to true when copy_file_range syscall is available
	// 2. That isCopyFileRangeSyscallAvailable() works on "new" kernels
	v, err := kernel.GetKernelVersion()
	require.NoError(t, err)

	copyWithFileRange := true
	copyWithFileClone := false
	doCopyTest(t, &copyWithFileRange, &copyWithFileClone)

	if kernel.CompareKernelVersion(*v, kernel.VersionInfo{Kernel: 4, Major: 5, Minor: 0}) < 0 {
		assert.False(t, copyWithFileRange)
	} else {
		assert.True(t, copyWithFileRange)
	}

}

func TestCopy(t *testing.T) {
	copyWithFileRange := true
	copyWithFileClone := true
	doCopyTest(t, &copyWithFileRange, &copyWithFileClone)
}

func TestCopyWithoutRange(t *testing.T) {
	copyWithFileRange := false
	copyWithFileClone := false
	doCopyTest(t, &copyWithFileRange, &copyWithFileClone)
}

func doCopyTest(t *testing.T, copyWithFileRange, copyWithFileClone *bool) {
	dir, err := ioutil.TempDir("", "docker-copy-check")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srcFilename := filepath.Join(dir, "srcFilename")
	dstFilename := filepath.Join(dir, "dstilename")

	r := rand.New(rand.NewSource(0))
	buf := make([]byte, 1024)
	_, err = r.Read(buf)
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(srcFilename, buf, 0777))
	fileinfo, err := os.Stat(srcFilename)
	require.NoError(t, err)

	require.NoError(t, copyRegular(srcFilename, dstFilename, fileinfo, copyWithFileRange, copyWithFileClone))
	readBuf, err := ioutil.ReadFile(dstFilename)
	require.NoError(t, err)
	assert.Equal(t, buf, readBuf)
}
