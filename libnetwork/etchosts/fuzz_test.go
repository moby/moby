package etchosts

import (
	"os"
	"path/filepath"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzAdd(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		fileBytes, err := ff.GetBytes()
		if err != nil {
			return
		}
		noOfRecords, err := ff.GetInt()
		if err != nil {
			return
		}

		recs := make([]Record, 0)
		for i := 0; i < noOfRecords%40; i++ {
			r := Record{}
			err = ff.GenerateStruct(&r)
			if err != nil {
				return
			}
			recs = append(recs, r)
		}
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "testFile")
		err = os.WriteFile(testFile, fileBytes, 0o644)
		if err != nil {
			return
		}
		_ = Add(testFile, recs)
	})
}
