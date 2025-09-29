package progress

import (
	"io"

	"github.com/moby/moby/api/pkg/progress"
)

// Reader is a Reader with progress bar.
type Reader = progress.Reader

// NewProgressReader creates a new ProgressReader.
func NewProgressReader(in io.ReadCloser, out progress.Output, size int64, id, action string) *progress.Reader {
	return progress.NewProgressReader(in, out, size, id, action)
}
