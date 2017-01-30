package opts

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestInferVolumeType(t *testing.T) {
	for s, typ := range map[string]VolumeType{
		"/foo":                              ShortUnsplitVolume,
		"foo":                               ShortUnsplitVolume,
		"/foo:/bar":                         ShortSplitVolume,
		"foo:/bar":                          ShortSplitVolume,
		"//c/foo:/bar":                      ShortSplitVolume,
		"c:/foo:/bar":                       ShortSplitVolume,
		"type=volume,src=foo,target=/bar":   LongVolume,
		"type=volume, src=foo, target=/bar": LongVolume,
		"type=blah":                         LongVolume,
	} {
		assert.Equal(t, InferVolumeType(s), typ)
	}
}
