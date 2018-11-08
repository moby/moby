package contextstore

import (
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/assert"
)

var testMetadata = ContextMetadata{
	Metadata: map[string]interface{}{
		"baz": "foo",
	},
	Endpoints: map[string]EndpointMetadata{
		"test-ep": {
			"foo": "bar",
		},
	},
}

func TestMetadataGetNotExisting(t *testing.T) {
	testDir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := metadataStore{root: testDir}
	_, err = testee.get("noexist")
	assert.Assert(t, os.IsNotExist(err))
}

func TestMetadataCreateGetRemove(t *testing.T) {
	testDir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := metadataStore{root: testDir}
	expected2 := ContextMetadata{
		Metadata: map[string]interface{}{
			"baz": "foo",
		},
		Endpoints: map[string]EndpointMetadata{
			"test-ep": {
				"foo": "bar",
			},
			"test-ep2": {
				"foo": "bar",
			},
		},
	}
	err = testee.createOrUpdate("test-context", testMetadata)
	assert.NilError(t, err)
	// create a new instance to check it does not depend on some sort of state
	testee = metadataStore{root: testDir}
	meta, err := testee.get("test-context")
	assert.NilError(t, err)
	assert.DeepEqual(t, meta, testMetadata)

	// update

	err = testee.createOrUpdate("test-context", expected2)
	assert.NilError(t, err)
	meta, err = testee.get("test-context")
	assert.NilError(t, err)
	assert.DeepEqual(t, meta, expected2)

	assert.NilError(t, testee.remove("test-context"))
	assert.NilError(t, testee.remove("test-context")) // support duplicate remove
	_, err = testee.get("test-context")
	assert.Assert(t, os.IsNotExist(err))
}

func TestMetadataList(t *testing.T) {
	testDir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := metadataStore{root: testDir}
	wholeData := map[string]ContextMetadata{
		"simple": {
			Metadata: map[string]interface{}{"foo": "bar"},
		},
		"simple2": {
			Metadata: map[string]interface{}{"foo": "bar"},
		},
		"nested/context": {
			Metadata: map[string]interface{}{"foo": "bar"},
		},
		"nestedwith-parent/context": {
			Metadata: map[string]interface{}{"foo": "bar"},
		},
		"nestedwith-parent": {
			Metadata: map[string]interface{}{"foo": "bar"},
		},
	}

	for k, s := range wholeData {
		err = testee.createOrUpdate(k, s)
		assert.NilError(t, err)
	}

	data, err := testee.list()
	assert.NilError(t, err)
	assert.DeepEqual(t, data, wholeData)
}
