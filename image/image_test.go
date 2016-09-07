package image

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/go-check/check"
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

func (s *DockerSuite) TestJSON(c *check.C) {
	img, err := NewFromJSON([]byte(sampleImageJSON))
	if err != nil {
		c.Fatal(err)
	}
	rawJSON := img.RawJSON()
	if string(rawJSON) != sampleImageJSON {
		c.Fatalf("Raw JSON of config didn't match: expected %+v, got %v", sampleImageJSON, rawJSON)
	}
}

func (s *DockerSuite) TestInvalidJSON(c *check.C) {
	_, err := NewFromJSON([]byte("{}"))
	if err == nil {
		c.Fatal("Expected JSON parse error")
	}
}

func (s *DockerSuite) TestMarshalKeyOrder(c *check.C) {
	b, err := json.Marshal(&Image{
		V1Image: V1Image{
			Comment:      "a",
			Author:       "b",
			Architecture: "c",
		},
	})
	if err != nil {
		c.Fatal(err)
	}

	expectedOrder := []string{"architecture", "author", "comment"}
	var indexes []int
	for _, k := range expectedOrder {
		indexes = append(indexes, strings.Index(string(b), k))
	}

	if !sort.IntsAreSorted(indexes) {
		c.Fatal("invalid key order in JSON: ", string(b))
	}
}
