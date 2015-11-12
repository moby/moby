package streams

import (
	"io"

	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/ioutils"
)

// StreamConfig groups several stream pipes for container I/O.
// StreamConfig.StdinPipe returns a WriteCloser which can be used to feed data
// to the standard input of the container's active process.
// Container.StdoutPipe and Container.StderrPipe each return a ReadCloser
// which can be used to retrieve the standard output (and error) generated
// by the container's active process. The output (and error) are actually
// copied and delivered to all StdoutPipe and StderrPipe consumers, using
// a kind of "broadcaster".
type StreamConfig struct {
	Stdout    *broadcaster.Unbuffered
	Stderr    *broadcaster.Unbuffered
	Stdin     io.ReadCloser
	StdinPipe io.WriteCloser
}

// StdoutPipe returns a new buffered pipe to read from the standard output.
func (streamConfig *StreamConfig) StdoutPipe() io.ReadCloser {
	reader, writer := io.Pipe()
	streamConfig.Stdout.Add(writer)
	return ioutils.NewBufReader(reader)
}

// StderrPipe returns a new buffered pipe to read from the standard error.
func (streamConfig *StreamConfig) StderrPipe() io.ReadCloser {
	reader, writer := io.Pipe()
	streamConfig.Stderr.Add(writer)
	return ioutils.NewBufReader(reader)
}
