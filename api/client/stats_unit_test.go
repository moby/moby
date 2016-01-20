package client

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/docker/engine-api/types"
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
		BlockReadByte:    100 * 1024 * 1024,
		BlockWriteByte:   800 * 1024 * 1024,
		BlockReadRate:    100 * 1024 * 1024,
		BlockWriteRate:   800 * 1024 * 1024,
		BlockReadIOPS:    10.00,
		BlockWriteIOPS:   8.00,
		mu:               sync.RWMutex{},
	}
	var b bytes.Buffer
	if err := c.Display(&b); err != nil {
		t.Fatalf("c.Display() gave error: %s", err)
	}
	got := b.String()
	want := "app\t30.00%\t104.9 MB / 2.147 GB\t4.88%\t104.9 MB / 838.9 MB\t104.9 MB / 838.9 MB\t104.9 MB / 838.9 MB\t10.00 / 8.00\n"
	if got != want {
		t.Fatalf("c.Display() = %q, want %q", got, want)
	}
}

func TestCalculBlockIO(t *testing.T) {
	preblkio := types.BlkioStats{
		IoServiceBytesRecursive: []types.BlkioStatEntry{{8, 0, "read", 1200}, {8, 1, "read", 4522}, {8, 0, "write", 112}, {8, 1, "write", 435}},
		IoServicedRecursive:     []types.BlkioStatEntry{{8, 0, "read", 12}, {8, 1, "read", 45}, {8, 0, "write", 11}, {8, 1, "write", 43}},
	}
	blkio := types.BlkioStats{
		IoServiceBytesRecursive: []types.BlkioStatEntry{{8, 0, "read", 1234}, {8, 1, "read", 4567}, {8, 0, "write", 123}, {8, 1, "write", 456}},
		IoServicedRecursive:     []types.BlkioStatEntry{{8, 0, "read", 13}, {8, 1, "read", 67}, {8, 0, "write", 23}, {8, 1, "write", 56}},
	}
	preread := time.Now()
	read := preread.Add(time.Second)

	blkReadByte, blkWriteByte, blkReadRate, blkWriteRate, blkReadIOPS, blkWriteIOPS := calculateBlockIO(blkio, preblkio, read, preread)
	if blkReadByte != 5801 {
		t.Fatalf("blkReadByte = %d, want 5801", blkReadByte)
	}
	if blkWriteByte != 579 {
		t.Fatalf("blkWriteByte = %d, want 579", blkWriteByte)
	}
	if blkReadRate != 79 {
		t.Fatalf("blkReadRate = %d, want 79", blkReadRate)
	}
	if blkWriteRate != 32 {
		t.Fatalf("blkWriteRate = %d, want 32", blkWriteRate)
	}
	if blkReadIOPS != 23.00 {
		t.Fatalf("blkReadIOPS = %.2f, want 23.00", blkReadIOPS)
	}
	if blkWriteIOPS != 25.00 {
		t.Fatalf("blkWriteIOPS = %.2f, want 25.00", blkWriteIOPS)
	}
}
