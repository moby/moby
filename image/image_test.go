package image

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

const sampleImageJSON = `{
	"architecture": "amd64",
	"os": "linux",
	"config": {},
	"rootfs": {
		"type": "layers",
		"diff_ids": []
	}
}`

func TestNewFromJSON(t *testing.T) {
	img, err := NewFromJSON([]byte(sampleImageJSON))
	assert.NilError(t, err)
	assert.Equal(t, string(img.RawJSON()), sampleImageJSON)
}

func TestNewFromJSONWithInvalidJSON(t *testing.T) {
	_, err := NewFromJSON([]byte("{}"))
	assert.Error(t, err, "invalid image JSON, no RootFS key")
}

func TestMarshalKeyOrder(t *testing.T) {
	b, err := json.Marshal(&Image{
		V1Image: V1Image{
			Comment:      "a",
			Author:       "b",
			Architecture: "c",
		},
	})
	assert.NilError(t, err)

	expectedOrder := []string{"architecture", "author", "comment"}
	var indexes []int
	for _, k := range expectedOrder {
		indexes = append(indexes, strings.Index(string(b), k))
	}

	if !sort.IntsAreSorted(indexes) {
		t.Fatal("invalid key order in JSON: ", string(b))
	}
}
