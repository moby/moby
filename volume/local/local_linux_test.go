//go:build linux
// +build linux

package local // import "github.com/docker/docker/volume/local"

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/docker/docker/errdefs"
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
	assert.NilError(t, os.WriteFile(testfile, make([]byte, quotaSize/2), 0644))
	assert.NilError(t, os.Remove(testfile))

	// test writing fiel larger than quota
	err = os.WriteFile(testfile, make([]byte, quotaSize+1), 0644)
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

func TestVolCreateValidation(t *testing.T) {
	r, err := New(t.TempDir(), idtools.Identity{UID: os.Geteuid(), GID: os.Getegid()})
	if err != nil {
		t.Fatal(err)
	}

	mandatoryOpts = map[string][]string{
		"device": {"type"},
		"type":   {"device"},
		"o":      {"device", "type"},
	}

	tests := []struct {
		doc         string
		name        string
		opts        map[string]string
		expectedErr string
	}{
		{
			doc:  "invalid: name too short",
			name: "a",
			opts: map[string]string{
				"type":   "foo",
				"device": "foo",
			},
			expectedErr: `volume name is too short, names should be at least two alphanumeric characters`,
		},
		{
			doc:  "invalid: name invalid characters",
			name: "hello world",
			opts: map[string]string{
				"type":   "foo",
				"device": "foo",
			},
			expectedErr: `"hello world" includes invalid characters for a local volume name, only "[a-zA-Z0-9][a-zA-Z0-9_.-]" are allowed. If you intended to pass a host directory, use absolute path`,
		},
		{
			doc:         "invalid: size, but no quotactl",
			opts:        map[string]string{"size": "1234"},
			expectedErr: `quota size requested but no quota support`,
		},
		{
			doc: "invalid: device without type",
			opts: map[string]string{
				"device": "foo",
			},
			expectedErr: `missing required option: "type"`,
		},
		{
			doc: "invalid: type without device",
			opts: map[string]string{
				"type": "foo",
			},
			expectedErr: `missing required option: "device"`,
		},
		{
			doc: "invalid: o without device",
			opts: map[string]string{
				"o":    "foo",
				"type": "foo",
			},
			expectedErr: `missing required option: "device"`,
		},
		{
			doc: "invalid: o without type",
			opts: map[string]string{
				"o":      "foo",
				"device": "foo",
			},
			expectedErr: `missing required option: "type"`,
		},
		{
			doc:  "valid: short name, no options",
			name: "ab",
		},
		{
			doc: "valid: device and type",
			opts: map[string]string{
				"type":   "foo",
				"device": "foo",
			},
		},
		{
			doc: "valid: device, type, and o",
			opts: map[string]string{
				"type":   "foo",
				"device": "foo",
				"o":      "foo",
			},
		},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			if tc.name == "" {
				tc.name = "vol-" + strconv.Itoa(i)
			}
			v, err := r.Create(tc.name, tc.opts)
			if v != nil {
				defer assert.Check(t, r.Remove(v))
			}
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Check(t, errdefs.IsInvalidParameter(err), "got: %T", err)
				assert.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}
