package tailfile

import (
	"os"
	"path/filepath"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzTailfile(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}
		ff := fuzz.NewConsumer(data)
		n, err := ff.GetUint64()
		if err != nil {
			return
		}
		fileBytes, err := ff.GetBytes()
		if err != nil {
			return
		}
		tempDir := t.TempDir()
		fil, err := os.Create(filepath.Join(tempDir, "tailFile"))
		if err != nil {
			return
		}
		defer fil.Close()

		_, err = fil.Write(fileBytes)
		if err != nil {
			return
		}
		fil.Seek(0, 0)
		_, _ = TailFile(fil, int(n))
	})
}
