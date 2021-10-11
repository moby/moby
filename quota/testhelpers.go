//go:build linux && !exclude_disk_quota && cgo
// +build linux,!exclude_disk_quota,cgo

package quota // import "github.com/docker/docker/quota"

import (
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

const imageSize = 64 * 1024 * 1024

// CanTestQuota - checks if xfs prjquota can be tested
// returns a reason if not
func CanTestQuota() (string, bool) {
	if os.Getuid() != 0 {
		return "requires mounts", false
	}
	_, err := exec.LookPath("mkfs.xfs")
	if err != nil {
		return "mkfs.xfs not found in PATH", false
	}
	return "", true
}

// PrepareQuotaTestImage - prepares an xfs prjquota test image
// returns the path the the image on success
func PrepareQuotaTestImage(t *testing.T) (string, error) {
	mkfs, err := exec.LookPath("mkfs.xfs")
	if err != nil {
		return "", err
	}

	// create a sparse image
	imageFile, err := os.CreateTemp("", "xfs-image")
	if err != nil {
		return "", err
	}
	imageFileName := imageFile.Name()
	if _, err = imageFile.Seek(imageSize-1, 0); err != nil {
		os.Remove(imageFileName)
		return "", err
	}
	if _, err = imageFile.Write([]byte{0}); err != nil {
		os.Remove(imageFileName)
		return "", err
	}
	if err = imageFile.Close(); err != nil {
		os.Remove(imageFileName)
		return "", err
	}

	// The reason for disabling these options is sometimes people run with a newer userspace
	// than kernelspace
	out, err := exec.Command(mkfs, "-m", "crc=0,finobt=0", imageFileName).CombinedOutput()
	if len(out) > 0 {
		t.Log(string(out))
	}
	if err != nil {
		os.Remove(imageFileName)
		return "", err
	}

	return imageFileName, nil
}

// WrapMountTest - wraps a test function such that it has easy access to a mountPoint and testDir
// with guaranteed prjquota or guaranteed no prjquota support.
func WrapMountTest(imageFileName string, enableQuota bool, testFunc func(t *testing.T, mountPoint, backingFsDev, testDir string)) func(*testing.T) {
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

		testDir, err := os.MkdirTemp(mountPoint, "per-test")
		assert.NilError(t, err)
		defer os.RemoveAll(testDir)

		testFunc(t, mountPoint, backingFsDev, testDir)
	}
}

// WrapQuotaTest - wraps a test function such that is has easy and guaranteed access to a quota Control
// instance with a quota test dir under its control.
func WrapQuotaTest(testFunc func(t *testing.T, ctrl *Control, mountPoint, testDir, testSubDir string)) func(t *testing.T, mountPoint, backingFsDev, testDir string) {
	return func(t *testing.T, mountPoint, backingFsDev, testDir string) {
		ctrl, err := NewControl(testDir)
		assert.NilError(t, err)

		testSubDir, err := os.MkdirTemp(testDir, "quota-test")
		assert.NilError(t, err)
		testFunc(t, ctrl, mountPoint, testDir, testSubDir)
	}
}
