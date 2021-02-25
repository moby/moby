// +build !windows
package stdio

import (
	"context"
	"io"
	"os"

	"github.com/containerd/fifo"
	"golang.org/x/sys/unix"
)

func openReader(ctx context.Context, p string) (io.ReadCloser, error) {
	return fifo.OpenFifo(ctx, p, unix.O_NONBLOCK|os.O_RDONLY, 0600)
}

func openWriter(ctx context.Context, p string) (io.WriteCloser, error) {
	return fifo.OpenFifo(ctx, p, unix.O_NONBLOCK|os.O_WRONLY, 0600)
}
