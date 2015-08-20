package client

import (
	"bytes"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestDisplay(t *testing.T) {
	c := &containerStats{
		Name:             "app",
		CPUPercentage:    30.0,
		Memory:           100 * 1024 * 1024.0,
		MemoryLimit:      2048 * 1024 * 1024.0,
		MemoryPercentage: 100.0 / 2048.0 * 100.0,
		NetworkRx:        100 * 1024 * 1024,
		NetworkTx:        800 * 1024 * 1024,
		BlockRead:        100 * 1024 * 1024,
		BlockWrite:       800 * 1024 * 1024,
		mu:               sync.RWMutex{},
	}
	var b bytes.Buffer
	if err := c.Display(&b); err != nil {
		t.Fatalf("c.Display() gave error: %s", err)
	}
	got := b.String()
	want := "app\t30.00%\t104.9 MB / 2.147 GB\t4.88%\t104.9 MB / 838.9 MB\t104.9 MB / 838.9 MB\n"
	if got != want {
		t.Fatalf("c.Display() = %q, want %q", got, want)
	}
}

func TestCalculBlockIO(t *testing.T) {
	blkio := types.BlkioStats{
		IoServiceBytesRecursive: []types.BlkioStatEntry{{8, 0, "read", 1234}, {8, 1, "read", 4567}, {8, 0, "write", 123}, {8, 1, "write", 456}},
	}
	blkRead, blkWrite := calculateBlockIO(blkio)
	if blkRead != 5801 {
		t.Fatalf("blkRead = %d, want 5801", blkRead)
	}
	if blkWrite != 579 {
		t.Fatalf("blkWrite = %d, want 579", blkWrite)
	}
}
