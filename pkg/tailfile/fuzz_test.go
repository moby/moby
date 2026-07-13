package tailfile

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func FuzzTailFile(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileBytes []byte, n uint64) {
		tempDir := t.TempDir()
		fil, err := os.Create(filepath.Join(tempDir, "tailFile"))
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		defer fil.Close()

		_, err = fil.Write(fileBytes)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fil.Seek(0, io.SeekStart)
		if err != nil {
			t.Fatal(err)
		}

		// Exercise TailFile with arbitrary inputs. Errors are expected for some
		// inputs; the fuzz target is only checking that it doesn't panic.
		_, _ = TailFile(fil, int(n))
	})
}
