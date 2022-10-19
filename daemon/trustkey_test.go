package daemon // import "github.com/docker/docker/daemon"

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// LoadOrCreateTrustKey
func TestLoadOrCreateTrustKeyInvalidKeyFile(t *testing.T) {
	tmpKeyFile, err := os.CreateTemp(t.TempDir(), "keyfile")
	assert.NilError(t, err)
	_ = tmpKeyFile.Close()

	_, err = loadOrCreateTrustKey(tmpKeyFile.Name())
	assert.Check(t, is.ErrorContains(err, "error loading key file"))
}

func TestLoadOrCreateTrustKeyCreateKeyWhenFileDoesNotExist(t *testing.T) {
	tmpKeyFile := filepath.Join(t.TempDir(), "keyfile")

	key, err := loadOrCreateTrustKey(tmpKeyFile)
	assert.NilError(t, err)
	assert.Check(t, key != nil)

	_, err = os.Stat(tmpKeyFile)
	assert.NilError(t, err, "key file doesn't exist")
}

func TestLoadOrCreateTrustKeyCreateKeyWhenDirectoryDoesNotExist(t *testing.T) {
	tmpKeyFile := filepath.Join(t.TempDir(), "folder/hierarchy/keyfile")
	key, err := loadOrCreateTrustKey(tmpKeyFile)
	assert.NilError(t, err)
	assert.Check(t, key != nil)

	_, err = os.Stat(tmpKeyFile)
	assert.NilError(t, err, "key file doesn't exist")
}

func TestLoadOrCreateTrustKeyCreateKeyNoPath(t *testing.T) {
	defer os.Remove("keyfile")
	key, err := loadOrCreateTrustKey("keyfile")
	assert.NilError(t, err)
	assert.Check(t, key != nil)

	_, err = os.Stat("keyfile")
	assert.NilError(t, err, "key file doesn't exist")
}

func TestLoadOrCreateTrustKeyLoadValidKey(t *testing.T) {
	tmpKeyFile := filepath.Join("testdata", "keyfile")
	key, err := loadOrCreateTrustKey(tmpKeyFile)
	assert.NilError(t, err)
	expected := "AWX2:I27X:WQFX:IOMK:CNAK:O7PW:VYNB:ZLKC:CVAE:YJP2:SI4A:XXAY"
	assert.Check(t, is.Contains(key.String(), expected))
}
