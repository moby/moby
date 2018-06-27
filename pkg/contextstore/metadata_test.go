package contextstore

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/assert"
)

func TestMetadataMarshalling(t *testing.T) {
	var ctxMeta ContextMetadata
	expected := ContextMetadata{
		Metadata: map[string]interface{}{
			"baz": "foo",
		},
		Endpoints: map[string]EndpointMetadata{
			"test-ep": {
				"foo": "bar",
			},
		},
	}
	bytes, err := json.Marshal(&expected)
	assert.NilError(t, err)
	err = json.Unmarshal(bytes, &ctxMeta)
	assert.NilError(t, err)
	assert.DeepEqual(t, ctxMeta, expected)
}

func TestMetadataGetNotExisting(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestMetadataGetNotExisting")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := metadataStore{root: testDir}
	_, err = testee.get("noexist")
	assert.Equal(t, true, os.IsNotExist(err))
}

func TestMetadataCreateGetRemove(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestMetadataCreateGet")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)
	testee := metadataStore{root: testDir}
	expected := ContextMetadata{
		Metadata: map[string]interface{}{
			"baz": "foo",
		},
		Endpoints: map[string]EndpointMetadata{
			"test-ep": {
				"foo": "bar",
			},
		},
	}
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
	err = testee.createOrUpdate("test-context", expected)
	assert.NilError(t, err)
	meta, err := testee.get("test-context")
	assert.NilError(t, err)
	assert.DeepEqual(t, meta, expected)

	// update

	err = testee.createOrUpdate("test-context", expected2)
	assert.NilError(t, err)
	meta, err = testee.get("test-context")
	assert.NilError(t, err)
	assert.DeepEqual(t, meta, expected2)

	assert.NilError(t, testee.remove("test-context"))
	assert.NilError(t, testee.remove("test-context")) // support duplicate remove
	_, err = testee.get("test-context")
	assert.Equal(t, true, os.IsNotExist(err))
}

func TestMetadataList(t *testing.T) {
	testDir, err := ioutil.TempDir("", "TestMetadataList")
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
