// +build linux

package copy // import "github.com/docker/docker/daemon/graphdriver/copy"

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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

func TestCopyDir(t *testing.T) {
	srcDir, err := ioutil.TempDir("", "srcDir")
	require.NoError(t, err)
	populateSrcDir(t, srcDir, 3)

	dstDir, err := ioutil.TempDir("", "testdst")
	require.NoError(t, err)
	defer os.RemoveAll(dstDir)

	assert.NoError(t, DirCopy(srcDir, dstDir, Content, false))
	require.NoError(t, filepath.Walk(srcDir, func(srcPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		relPath, err := filepath.Rel(srcDir, srcPath)
		require.NoError(t, err)
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath)
		require.NoError(t, err)

		// If we add non-regular dirs and files to the test
		// then we need to add more checks here.
		dstFileInfo, err := os.Lstat(dstPath)
		require.NoError(t, err)

		srcFileSys := f.Sys().(*syscall.Stat_t)
		dstFileSys := dstFileInfo.Sys().(*syscall.Stat_t)

		t.Log(relPath)
		if srcFileSys.Dev == dstFileSys.Dev {
			assert.NotEqual(t, srcFileSys.Ino, dstFileSys.Ino)
		}
		// Todo: check size, and ctim is not equal
		/// on filesystems that have granular ctimes
		assert.Equal(t, srcFileSys.Mode, dstFileSys.Mode)
		assert.Equal(t, srcFileSys.Uid, dstFileSys.Uid)
		assert.Equal(t, srcFileSys.Gid, dstFileSys.Gid)
		assert.Equal(t, srcFileSys.Mtim, dstFileSys.Mtim)

		return nil
	}))
}

func randomMode(baseMode int) os.FileMode {
	for i := 0; i < 7; i++ {
		baseMode = baseMode | (1&rand.Intn(2))<<uint(i)
	}
	return os.FileMode(baseMode)
}

func populateSrcDir(t *testing.T, srcDir string, remainingDepth int) {
	if remainingDepth == 0 {
		return
	}
	aTime := time.Unix(rand.Int63(), 0)
	mTime := time.Unix(rand.Int63(), 0)

	for i := 0; i < 10; i++ {
		dirName := filepath.Join(srcDir, fmt.Sprintf("srcdir-%d", i))
		// Owner all bits set
		require.NoError(t, os.Mkdir(dirName, randomMode(0700)))
		populateSrcDir(t, dirName, remainingDepth-1)
		require.NoError(t, system.Chtimes(dirName, aTime, mTime))
	}

	for i := 0; i < 10; i++ {
		fileName := filepath.Join(srcDir, fmt.Sprintf("srcfile-%d", i))
		// Owner read bit set
		require.NoError(t, ioutil.WriteFile(fileName, []byte{}, randomMode(0400)))
		require.NoError(t, system.Chtimes(fileName, aTime, mTime))
	}
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
