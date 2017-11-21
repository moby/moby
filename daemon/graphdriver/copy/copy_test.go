// +build linux

package copy

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

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

func TestCopyHardlink(t *testing.T) {
	var srcFile1FileInfo, srcFile2FileInfo, dstFile1FileInfo, dstFile2FileInfo unix.Stat_t

	srcDir, err := ioutil.TempDir("", "srcDir")
	require.NoError(t, err)
	defer os.RemoveAll(srcDir)

	dstDir, err := ioutil.TempDir("", "dstDir")
	require.NoError(t, err)
	defer os.RemoveAll(dstDir)

	srcFile1 := filepath.Join(srcDir, "file1")
	srcFile2 := filepath.Join(srcDir, "file2")
	dstFile1 := filepath.Join(dstDir, "file1")
	dstFile2 := filepath.Join(dstDir, "file2")
	require.NoError(t, ioutil.WriteFile(srcFile1, []byte{}, 0777))
	require.NoError(t, os.Link(srcFile1, srcFile2))

	assert.NoError(t, DirCopy(srcDir, dstDir, Content, false))

	require.NoError(t, unix.Stat(srcFile1, &srcFile1FileInfo))
	require.NoError(t, unix.Stat(srcFile2, &srcFile2FileInfo))
	require.Equal(t, srcFile1FileInfo.Ino, srcFile2FileInfo.Ino)

	require.NoError(t, unix.Stat(dstFile1, &dstFile1FileInfo))
	require.NoError(t, unix.Stat(dstFile2, &dstFile2FileInfo))
	assert.Equal(t, dstFile1FileInfo.Ino, dstFile2FileInfo.Ino)
}
