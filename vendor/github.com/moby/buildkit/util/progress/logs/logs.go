package logs

import (
	"context"
	"io"
	"os"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/progress"
	"github.com/pkg/errors"
)

func NewLogStreams(ctx context.Context, printOutput bool) (io.WriteCloser, io.WriteCloser) {
	return newStreamWriter(ctx, 1, printOutput), newStreamWriter(ctx, 2, printOutput)
}

func newStreamWriter(ctx context.Context, stream int, printOutput bool) io.WriteCloser {
	pw, _, _ := progress.FromContext(ctx)
	return &streamWriter{
		pw:          pw,
		stream:      stream,
		printOutput: printOutput,
	}
}

type streamWriter struct {
	pw          progress.Writer
	stream      int
	printOutput bool
}

func (sw *streamWriter) Write(dt []byte) (int, error) {
	sw.pw.Write(identity.NewID(), client.VertexLog{
		Stream: sw.stream,
		Data:   append([]byte{}, dt...),
	})
	if sw.printOutput {
		switch sw.stream {
		case 1:
			return os.Stdout.Write(dt)
		case 2:
			return os.Stderr.Write(dt)
		default:
			return 0, errors.Errorf("invalid stream %d", sw.stream)
		}
	}
	return len(dt), nil
}

func (sw *streamWriter) Close() error {
	return sw.pw.Close()
}
