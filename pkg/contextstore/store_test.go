package contextstore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
)

func TestStoreInitScratch(t *testing.T) {
	dirname, err := ioutil.TempDir("", "test-store-init-scratch")
	assert.NilError(t, err)
	defer os.RemoveAll(dirname)
	testDir := filepath.Join(dirname, "test")
	_, err = NewStore(testDir)
	assert.NilError(t, err)
	metaFS, err := os.Stat(filepath.Join(testDir, metadataDir))
	assert.NilError(t, err)
	assert.Equal(t, metaFS.IsDir(), true)
	tlsFS, err := os.Stat(filepath.Join(testDir, tlsDir))
	assert.NilError(t, err)
	assert.Equal(t, tlsFS.IsDir(), true)

	// check that we can create a store from existing dir
	_, err = NewStore(testDir)
	assert.NilError(t, err)
}

func TestSetGetCurrentContext(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestSetGetCurrentContext")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	store1, err := NewStore(testDir)
	assert.NilError(t, err)
	err = store1.SetCurrentContext("test")
	assert.NilError(t, err)
	store2, err := NewStore(testDir)
	assert.NilError(t, err)
	assert.Equal(t, "test", store2.GetCurrentContext())
}

func TestExportImport(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestSetGetCurrentContext")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	s, err := NewStore(testDir)
	assert.NilError(t, err)
	err = s.CreateOrUpdateContext("source",
		ContextMetadata{
			Endpoints: map[string]EndpointMetadata{
				"ep1": map[string]interface{}{
					"foo": "bar",
				},
			},
			Metadata: map[string]interface{}{
				"bar": "baz",
			},
		})
	assert.NilError(t, err)
	err = s.ResetContextEndpointTLSMaterial("source", "ep1", &EndpointTLSData{
		Files: map[string][]byte{
			"file1": []byte("test-data"),
		},
	})
	assert.NilError(t, err)
	r := Export("source", s)
	defer r.Close()
	err = Import("dest", s, r)
	assert.NilError(t, err)
	srcMeta, err := s.GetContextMetadata("source")
	assert.NilError(t, err)
	destMeta, err := s.GetContextMetadata("dest")
	assert.NilError(t, err)
	assert.DeepEqual(t, destMeta, srcMeta)
	srcFileList, err := s.ListContextTLSFiles("source")
	assert.NilError(t, err)
	destFileList, err := s.ListContextTLSFiles("dest")
	assert.NilError(t, err)
	assert.DeepEqual(t, srcFileList, destFileList)
	srcData, err := s.GetContextTLSData("source", "ep1", "file1")
	assert.NilError(t, err)
	assert.Equal(t, "test-data", string(srcData))
	destData, err := s.GetContextTLSData("dest", "ep1", "file1")
	assert.NilError(t, err)
	assert.Equal(t, "test-data", string(destData))
}

func TestRemove(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestRemove")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	s, err := NewStore(testDir)
	assert.NilError(t, err)
	err = s.CreateOrUpdateContext("source",
		ContextMetadata{
			Endpoints: map[string]EndpointMetadata{
				"ep1": map[string]interface{}{
					"foo": "bar",
				},
			},
			Metadata: map[string]interface{}{
				"bar": "baz",
			},
		})
	assert.NilError(t, err)
	assert.NilError(t, s.ResetContextEndpointTLSMaterial("source", "ep1", &EndpointTLSData{
		Files: map[string][]byte{
			"file1": []byte("test-data"),
		},
	}))
	assert.NilError(t, s.RemoveContext("source"))
	_, err = s.GetContextMetadata("source")
	assert.Check(t, os.IsNotExist(err))
	f, err := s.ListContextTLSFiles("source")
	assert.NilError(t, err)
	assert.Equal(t, 0, len(f))
}
