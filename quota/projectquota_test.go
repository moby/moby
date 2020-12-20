// +build linux

package quota // import "github.com/docker/docker/quota"

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// 10MB
const testQuotaSize = 10 * 1024 * 1024

func TestBlockDev(t *testing.T) {
	if msg, ok := CanTestQuota(); !ok {
		t.Skip(msg)
	}

	// get sparse xfs test image
	imageFileName, err := PrepareQuotaTestImage(t)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(imageFileName)

	t.Run("testBlockDevQuotaDisabled", WrapMountTest(imageFileName, false, testBlockDevQuotaDisabled))
	t.Run("testBlockDevQuotaEnabled", WrapMountTest(imageFileName, true, testBlockDevQuotaEnabled))
	t.Run("testSmallerThanQuota", WrapMountTest(imageFileName, true, WrapQuotaTest(testSmallerThanQuota)))
	t.Run("testBiggerThanQuota", WrapMountTest(imageFileName, true, WrapQuotaTest(testBiggerThanQuota)))
	t.Run("testRetrieveQuota", WrapMountTest(imageFileName, true, WrapQuotaTest(testRetrieveQuota)))
}

func testBlockDevQuotaDisabled(t *testing.T, mountPoint, backingFsDev, testDir string) {
	hasSupport, err := hasQuotaSupport(backingFsDev)
	assert.NilError(t, err)
	assert.Check(t, !hasSupport)
}

func testBlockDevQuotaEnabled(t *testing.T, mountPoint, backingFsDev, testDir string) {
	hasSupport, err := hasQuotaSupport(backingFsDev)
	assert.NilError(t, err)
	assert.Check(t, hasSupport)
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
