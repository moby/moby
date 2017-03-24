package container

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestCalculateBlockIO(t *testing.T) {
	blkio := types.BlkioStats{
		IoServiceBytesRecursive: []types.BlkioStatEntry{{Major: 8, Minor: 0, Op: "read", Value: 1234}, {Major: 8, Minor: 1, Op: "read", Value: 4567}, {Major: 8, Minor: 0, Op: "write", Value: 123}, {Major: 8, Minor: 1, Op: "write", Value: 456}},
	}
	blkRead, blkWrite := calculateBlockIO(blkio)
	if blkRead != 5801 {
		t.Fatalf("blkRead = %d, want 5801", blkRead)
	}
	if blkWrite != 579 {
		t.Fatalf("blkWrite = %d, want 579", blkWrite)
	}
}
