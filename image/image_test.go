package image

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, sampleImageJSON, string(img.RawJSON()))
}

func TestNewFromJSONWithInvalidJSON(t *testing.T) {
	_, err := NewFromJSON([]byte("{}"))
	assert.EqualError(t, err, "invalid image JSON, no RootFS key")
}

func TestMarshalKeyOrder(t *testing.T) {
	b, err := json.Marshal(&Image{
		V1Image: V1Image{
			Comment:      "a",
			Author:       "b",
			Architecture: "c",
		},
	})
	assert.NoError(t, err)

	expectedOrder := []string{"architecture", "author", "comment"}
	var indexes []int
	for _, k := range expectedOrder {
		indexes = append(indexes, strings.Index(string(b), k))
	}

	if !sort.IntsAreSorted(indexes) {
		t.Fatal("invalid key order in JSON: ", string(b))
	}
}
