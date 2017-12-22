// +build linux

package quota // import "github.com/docker/docker/daemon/graphdriver/quota"

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/fs"
	"golang.org/x/sys/unix"
)

// 10MB
const testQuotaSize = 10 * 1024 * 1024
const imageSize = 64 * 1024 * 1024

func TestBlockDev(t *testing.T) {
	mkfs, err := exec.LookPath("mkfs.xfs")
	if err != nil {
		t.Skip("mkfs.xfs not found in PATH")
	}

	// create a sparse image
	imageFile, err := ioutil.TempFile("", "xfs-image")
	if err != nil {
		t.Fatal(err)
	}
	imageFileName := imageFile.Name()
	defer os.Remove(imageFileName)
	if _, err = imageFile.Seek(imageSize-1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = imageFile.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	if err = imageFile.Close(); err != nil {
		t.Fatal(err)
	}

	// The reason for disabling these options is sometimes people run with a newer userspace
	// than kernelspace
	out, err := exec.Command(mkfs, "-m", "crc=0,finobt=0", imageFileName).CombinedOutput()
	if len(out) > 0 {
		t.Log(string(out))
	}
	if err != nil {
		t.Fatal(err)
	}

	t.Run("testBlockDevQuotaDisabled", wrapMountTest(imageFileName, false, testBlockDevQuotaDisabled))
	t.Run("testBlockDevQuotaEnabled", wrapMountTest(imageFileName, true, testBlockDevQuotaEnabled))
	t.Run("testSmallerThanQuota", wrapMountTest(imageFileName, true, wrapQuotaTest(testSmallerThanQuota)))
	t.Run("testBiggerThanQuota", wrapMountTest(imageFileName, true, wrapQuotaTest(testBiggerThanQuota)))
	t.Run("testRetrieveQuota", wrapMountTest(imageFileName, true, wrapQuotaTest(testRetrieveQuota)))
}

func wrapMountTest(imageFileName string, enableQuota bool, testFunc func(t *testing.T, mountPoint, backingFsDev string)) func(*testing.T) {
	return func(t *testing.T) {
		mountOptions := "loop"

		if enableQuota {
			mountOptions = mountOptions + ",prjquota"
		}

		mountPointDir := fs.NewDir(t, "xfs-mountPoint")
		defer mountPointDir.Remove()
		mountPoint := mountPointDir.Path()

		out, err := exec.Command("mount", "-o", mountOptions, imageFileName, mountPoint).CombinedOutput()
		if err != nil {
			_, err := os.Stat("/proc/fs/xfs")
			if os.IsNotExist(err) {
				t.Skip("no /proc/fs/xfs")
			}
		}

		assert.NilError(t, err, "mount failed: %s", out)

		defer func() {
			assert.NilError(t, unix.Unmount(mountPoint, 0))
		}()

		backingFsDev, err := makeBackingFsDev(mountPoint)
		assert.NilError(t, err)

		testFunc(t, mountPoint, backingFsDev)
	}
}

func testBlockDevQuotaDisabled(t *testing.T, mountPoint, backingFsDev string) {
	hasSupport, err := hasQuotaSupport(backingFsDev)
	assert.NilError(t, err)
	assert.Check(t, !hasSupport)
}

func testBlockDevQuotaEnabled(t *testing.T, mountPoint, backingFsDev string) {
	hasSupport, err := hasQuotaSupport(backingFsDev)
	assert.NilError(t, err)
	assert.Check(t, hasSupport)
}

func wrapQuotaTest(testFunc func(t *testing.T, ctrl *Control, mountPoint, testDir, testSubDir string)) func(t *testing.T, mountPoint, backingFsDev string) {
	return func(t *testing.T, mountPoint, backingFsDev string) {
		testDir, err := ioutil.TempDir(mountPoint, "per-test")
		assert.NilError(t, err)
		defer os.RemoveAll(testDir)

		ctrl, err := NewControl(testDir)
		assert.NilError(t, err)

		testSubDir, err := ioutil.TempDir(testDir, "quota-test")
		assert.NilError(t, err)
		testFunc(t, ctrl, mountPoint, testDir, testSubDir)
	}

}

func testSmallerThanQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	assert.NilError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))
	smallerThanQuotaFile := filepath.Join(testSubDir, "smaller-than-quota")
	assert.NilError(t, ioutil.WriteFile(smallerThanQuotaFile, make([]byte, testQuotaSize/2), 0644))
	assert.NilError(t, os.Remove(smallerThanQuotaFile))
}

func testBiggerThanQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	// Make sure the quota is being enforced
	// TODO: When we implement this under EXT4, we need to shed CAP_SYS_RESOURCE, otherwise
	// we're able to violate quota without issue
	assert.NilError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))

	biggerThanQuotaFile := filepath.Join(testSubDir, "bigger-than-quota")
	err := ioutil.WriteFile(biggerThanQuotaFile, make([]byte, testQuotaSize+1), 0644)
	assert.Assert(t, is.ErrorContains(err, ""))
	if err == io.ErrShortWrite {
		assert.NilError(t, os.Remove(biggerThanQuotaFile))
	}
}

func testRetrieveQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	// Validate that we can retrieve quota
	assert.NilError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))

	var q Quota
	assert.NilError(t, ctrl.GetQuota(testSubDir, &q))
	assert.Check(t, is.Equal(uint64(testQuotaSize), q.Size))
}
