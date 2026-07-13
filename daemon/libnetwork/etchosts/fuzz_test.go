package etchosts

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func FuzzAdd(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileBytes []byte, data []byte, noOfRecords uint8) {
		recs := make([]Record, 0)
		for range noOfRecords % 40 {
			if len(data) == 0 {
				break
			}
			r := Record{}
			data = generateRecord(data, &r)
			recs = append(recs, r)
		}
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "testFile")
		err := os.WriteFile(testFile, fileBytes, 0o644)
		if err != nil {
			t.Fatalf("write test hosts file: %v", err)
		}

		// Exercise Add with arbitrary file contents and records. Errors are
		// expected for some inputs; the fuzz target is only checking that it
		// doesn't panic.
		_ = Add(testFile, recs)
	})
}

func generateRecord(data []byte, r *Record) []byte {
	hostLen := int(data[0] % 64)
	data = data[1:]
	hostLen = min(hostLen, len(data))

	r.Hosts = string(data[:hostLen])
	data = data[hostLen:]

	if len(data) >= 4 {
		r.IP = netip.AddrFrom4([4]byte(data[:4]))
		data = data[4:]
	}

	return data
}
