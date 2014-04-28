package fs

import (
	"testing"
)

const (
	sectorsRecursiveContents      = `8:0 1024`
	serviceBytesRecursiveContents = `8:0 Read 100
8:0 Write 400
8:0 Sync 200
8:0 Async 300
8:0 Total 500
Total 500`
	servicedRecursiveContents = `8:0 Read 10
8:0 Write 40
8:0 Sync 20
8:0 Async 30
8:0 Total 50
Total 50`
	queuedRecursiveContents = `8:0 Read 1
8:0 Write 4
8:0 Sync 2
8:0 Async 3
8:0 Total 5
Total 5`
)

func TestBlkioStats(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": serviceBytesRecursiveContents,
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	stats, err := blkio.Stats(helper.CgroupData)
	if err != nil {
		t.Fatal(err)
	}

	// Verify expected stats.
	expectedStats := map[string]float64{
		"blkio.sectors_recursive:8:0": 1024.0,

		// Serviced bytes.
		"io_service_bytes_recursive:8:0:Read":  100.0,
		"io_service_bytes_recursive:8:0:Write": 400.0,
		"io_service_bytes_recursive:8:0:Sync":  200.0,
		"io_service_bytes_recursive:8:0:Async": 300.0,
		"io_service_bytes_recursive:8:0:Total": 500.0,

		// Serviced requests.
		"io_serviced_recursive:8:0:Read":  10.0,
		"io_serviced_recursive:8:0:Write": 40.0,
		"io_serviced_recursive:8:0:Sync":  20.0,
		"io_serviced_recursive:8:0:Async": 30.0,
		"io_serviced_recursive:8:0:Total": 50.0,

		// Queued requests.
		"io_queued_recursive:8:0:Read":  1.0,
		"io_queued_recursive:8:0:Write": 4.0,
		"io_queued_recursive:8:0:Sync":  2.0,
		"io_queued_recursive:8:0:Async": 3.0,
		"io_queued_recursive:8:0:Total": 5.0,
	}
	expectStats(t, expectedStats, stats)
}

func TestBlkioStatsNoSectorsFile(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": serviceBytesRecursiveContents,
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsNoServiceBytesFile(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_serviced_recursive": servicedRecursiveContents,
		"blkio.io_queued_recursive":   queuedRecursiveContents,
		"blkio.sectors_recursive":     sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsNoServicedFile(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": serviceBytesRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsNoQueuedFile(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": serviceBytesRecursiveContents,
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsUnexpectedNumberOfFields(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": "8:0 Read 100 100",
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsUnexpectedFieldType(t *testing.T) {
	helper := NewCgroupTestUtil("blkio", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		"blkio.io_service_bytes_recursive": "8:0 Read Write",
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
	})

	blkio := &blkioGroup{}
	_, err := blkio.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}
