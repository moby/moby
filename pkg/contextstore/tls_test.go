package contextstore

import (
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/assert"
)

func TestTlsCreateUpdateGetRemove(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestTlsCreateUpdateGetRemove")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := tlsStore{root: testDir}
	_, err = testee.getData("test-ctx", "test-ep", "test-data")
	assert.Equal(t, true, os.IsNotExist(err))

	err = testee.createOrUpdate("test-ctx", "test-ep", "test-data", []byte("data"))
	assert.NilError(t, err)
	data, err := testee.getData("test-ctx", "test-ep", "test-data")
	assert.NilError(t, err)
	assert.Equal(t, string(data), "data")
	err = testee.createOrUpdate("test-ctx", "test-ep", "test-data", []byte("data2"))
	assert.NilError(t, err)
	data, err = testee.getData("test-ctx", "test-ep", "test-data")
	assert.NilError(t, err)
	assert.Equal(t, string(data), "data2")

	err = testee.remove("test-ctx", "test-ep", "test-data")
	assert.NilError(t, err)
	err = testee.remove("test-ctx", "test-ep", "test-data")
	assert.NilError(t, err)

	_, err = testee.getData("test-ctx", "test-ep", "test-data")
	assert.Equal(t, true, os.IsNotExist(err))
}

func TestTlsListAndBatchRemove(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestTlsListAndBatchRemove")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := tlsStore{root: testDir}

	all := map[string]EndpointFiles{
		"ep1": {"f1", "f2", "f3"},
		"ep2": {"f1", "f2", "f3"},
		"ep3": {"f1", "f2", "f3"},
	}

	ep1ep2 := map[string]EndpointFiles{
		"ep1": {"f1", "f2", "f3"},
		"ep2": {"f1", "f2", "f3"},
	}

	for name, files := range all {
		for _, file := range files {
			err = testee.createOrUpdate("test-ctx", name, file, []byte("data"))
			assert.NilError(t, err)
		}
	}

	resAll, err := testee.listContextData("test-ctx")
	assert.NilError(t, err)
	assert.DeepEqual(t, resAll, all)

	err = testee.removeAllEndpointData("test-ctx", "ep3")
	assert.NilError(t, err)
	resEp1ep2, err := testee.listContextData("test-ctx")
	assert.NilError(t, err)
	assert.DeepEqual(t, resEp1ep2, ep1ep2)

	err = testee.removeAllContextData("test-ctx")
	assert.NilError(t, err)
	resEmpty, err := testee.listContextData("test-ctx")
	assert.NilError(t, err)
	assert.DeepEqual(t, resEmpty, map[string]EndpointFiles{})

}
