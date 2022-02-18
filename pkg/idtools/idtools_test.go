package idtools // import "github.com/moby/moby/pkg/idtools"

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCreateIDMapOrder(t *testing.T) {
	subidRanges := ranges{
		{100000, 1000},
		{1000, 1},
	}

	idMap := createIDMap(subidRanges)
	assert.DeepEqual(t, idMap, []IDMap{
		{
			ContainerID: 0,
			HostID:      100000,
			Size:        1000,
		},
		{
			ContainerID: 1000,
			HostID:      1000,
			Size:        1,
		},
	})
}
