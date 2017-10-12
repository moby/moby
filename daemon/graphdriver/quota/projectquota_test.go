// +build linux

package quota

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 10MB
const testQuotaSize = 10 * 1024 * 1024
const nonDockerOwnedTestProjectID = 25000

func TestQuota(t *testing.T) {
	var err error
	homeDir, err := ioutil.TempDir("", "docker-copy-check")
	require.NoError(t, err)
	defer os.RemoveAll(homeDir)

	backingFsDev, err := makeBackingFsDev(homeDir)
	require.NoError(t, err)

	hasSupport, err := hasQuotaSupport(backingFsDev)
	require.NoError(t, err)

	if !hasSupport {
		// Do some minimal tests here, but point out to the user
		// that we weren't able to test fully (skip)
		ctrl, err := NewControl(homeDir)
		require.Nil(t, ctrl)
		require.Error(t, err)
		t.Skip("Quota not supported")
	}

	t.Run("testSmallerThanQuota", wrapTest(homeDir, testSmallerThanQuota))
	t.Run("testBiggerThanQuota", wrapTest(homeDir, testBiggerThanQuota))
	t.Run("testRetrieveQuota", wrapTest(homeDir, testRetrieveQuota))
	t.Run("testNoChangeUserQuota", wrapTest(homeDir, testNoChangeUserQuota))
	t.Run("testChangeQuota", wrapTest(homeDir, testChangeQuota))
}

func wrapTest(homeDir string, testFunc func(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string)) func(*testing.T) {
	return func(t *testing.T) {
		var err error
		testDir, err := ioutil.TempDir(homeDir, "per-test")
		defer os.RemoveAll(testDir)
		require.NoError(t, err)
		ctrl, err := NewControl(testDir)
		require.NoError(t, err)
		testSubDir, err := ioutil.TempDir(testDir, "quota-test")
		require.NoError(t, err)
		testFunc(t, ctrl, homeDir, testDir, testSubDir)
	}
}

func testSmallerThanQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	require.NoError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))
	smallerThanQuotaFile := filepath.Join(testSubDir, "smaller-than-quota")
	require.NoError(t, ioutil.WriteFile(smallerThanQuotaFile, make([]byte, testQuotaSize/2), 0644))
	require.NoError(t, os.Remove(smallerThanQuotaFile))
}

func testBiggerThanQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	// Make sure the quota is being enforced
	// TODO: When we implement this under EXT4, we need to shed CAP_SYS_RESOURCE, otherwise
	// we're able to violate quota without issue
	require.NoError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))

	biggerThanQuotaFile := filepath.Join(testSubDir, "bigger-than-quota")
	err := ioutil.WriteFile(biggerThanQuotaFile, make([]byte, testQuotaSize+1), 0644)
	require.Error(t, err)
	if err == io.ErrShortWrite {
		require.NoError(t, os.Remove(biggerThanQuotaFile))
	}
}

func testRetrieveQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	// Validate that we can retrieve quota
	require.NoError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))

	var q Quota
	require.NoError(t, ctrl.GetQuota(testSubDir, &q))
	assert.EqualValues(t, testQuotaSize, q.Size)
}

func testNoChangeUserQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	require.NoError(t, setProjectID(testSubDir, nonDockerOwnedTestProjectID))
	require.Error(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))
}

func testChangeQuota(t *testing.T, ctrl *Control, homeDir, testDir, testSubDir string) {
	require.NoError(t, ctrl.SetQuota(testSubDir, Quota{testQuotaSize}))
	ctrl2, err := NewControl(testDir)
	require.NoError(t, err)
	// 1. Can we get the quota back?
	// 2. Can we change it?
	var q Quota
	require.NoError(t, ctrl2.GetQuota(testSubDir, &q))
	assert.EqualValues(t, testQuotaSize, q.Size)

	require.NoError(t, ctrl2.SetQuota(testSubDir, Quota{testQuotaSize + 1}))
}
