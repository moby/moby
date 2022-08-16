package daemon // import "github.com/docker/docker/daemon"

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

// LoadOrCreateTrustKey
func TestLoadOrCreateTrustKeyInvalidKeyFile(t *testing.T) {
	tmpKeyFolderPath, err := os.MkdirTemp("", "api-trustkey-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpKeyFolderPath)

	tmpKeyFile, err := os.CreateTemp(tmpKeyFolderPath, "keyfile")
	assert.NilError(t, err)
	defer tmpKeyFile.Close()

	_, err = loadOrCreateTrustKey(tmpKeyFile.Name())
	assert.Check(t, is.ErrorContains(err, "Error loading key file"))
}

func TestLoadOrCreateTrustKeyCreateKeyWhenFileDoesNotExist(t *testing.T) {
	tmpKeyFolderPath := fs.NewDir(t, "api-trustkey-test")
	defer tmpKeyFolderPath.Remove()

	// Without the need to create the folder hierarchy
	tmpKeyFile := tmpKeyFolderPath.Join("keyfile")

	key, err := loadOrCreateTrustKey(tmpKeyFile)
	assert.NilError(t, err)
	assert.Check(t, key != nil)

	_, err = os.Stat(tmpKeyFile)
	assert.NilError(t, err, "key file doesn't exist")
}

func TestLoadOrCreateTrustKeyCreateKeyWhenDirectoryDoesNotExist(t *testing.T) {
	tmpKeyFolderPath := fs.NewDir(t, "api-trustkey-test")
	defer tmpKeyFolderPath.Remove()
	tmpKeyFile := tmpKeyFolderPath.Join("folder/hierarchy/keyfile")

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
