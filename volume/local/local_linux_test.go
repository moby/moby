//go:build linux
// +build linux

package local // import "github.com/docker/docker/volume/local"

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/quota"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const quotaSize = 1024 * 1024
const quotaSizeLiteral = "1M"

func TestQuota(t *testing.T) {
	if msg, ok := quota.CanTestQuota(); !ok {
		t.Skip(msg)
	}

	// get sparse xfs test image
	imageFileName, err := quota.PrepareQuotaTestImage(t)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(imageFileName)

	t.Run("testVolWithQuota", quota.WrapMountTest(imageFileName, true, testVolWithQuota))
	t.Run("testVolQuotaUnsupported", quota.WrapMountTest(imageFileName, false, testVolQuotaUnsupported))
}

func testVolWithQuota(t *testing.T, mountPoint, backingFsDev, testDir string) {
	r, err := New(testDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}
	assert.Assert(t, r.quotaCtl != nil)

	vol, err := r.Create("testing", map[string]string{"size": quotaSizeLiteral})
	if err != nil {
		t.Fatal(err)
	}

	dir, err := vol.Mount("1234")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := vol.Unmount("1234"); err != nil {
			t.Fatal(err)
		}
	}()

	testfile := filepath.Join(dir, "testfile")

	// test writing file smaller than quota
	assert.NilError(t, ioutil.WriteFile(testfile, make([]byte, quotaSize/2), 0644))
	assert.NilError(t, os.Remove(testfile))

	// test writing fiel larger than quota
	err = ioutil.WriteFile(testfile, make([]byte, quotaSize+1), 0644)
	assert.ErrorContains(t, err, "")
	if _, err := os.Stat(testfile); err == nil {
		assert.NilError(t, os.Remove(testfile))
	}
}

func testVolQuotaUnsupported(t *testing.T, mountPoint, backingFsDev, testDir string) {
	r, err := New(testDir, idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}
	assert.Assert(t, is.Nil(r.quotaCtl))

	_, err = r.Create("testing", map[string]string{"size": quotaSizeLiteral})
	assert.ErrorContains(t, err, "no quota support")

	vol, err := r.Create("testing", nil)
	if err != nil {
		t.Fatal(err)
	}

	// this could happen if someone moves volumes from storage with
	// quota support to some place without
	lv, ok := vol.(*localVolume)
	assert.Assert(t, ok)
	lv.opts = &optsConfig{
		Quota: quota.Quota{Size: quotaSize},
	}

	_, err = vol.Mount("1234")
	assert.ErrorContains(t, err, "no quota support")
}
