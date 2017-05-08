package runconfig

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/ioutils"
)

// StreamConfig holds information about I/O streams managed together.
//
// streamConfig.StdinPipe returns a WriteCloser which can be used to feed data
// to the standard input of the streamConfig's active process.
// streamConfig.StdoutPipe and streamConfig.StderrPipe each return a ReadCloser
// which can be used to retrieve the standard output (and error) generated
// by the container's active process. The output (and error) are actually
// copied and delivered to all StdoutPipe and StderrPipe consumers, using
// a kind of "broadcaster".
type StreamConfig struct {
	sync.WaitGroup
	stdout    *broadcaster.Unbuffered
	stderr    *broadcaster.Unbuffered
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
}

// NewStreamConfig creates a stream config and initializes
// the standard err and standard out to new unbuffered broadcasters.
func NewStreamConfig() *StreamConfig {
	return &StreamConfig{
		stderr: new(broadcaster.Unbuffered),
		stdout: new(broadcaster.Unbuffered),
	}
}

// Stdout returns the standard output in the configuration.
func (streamConfig *StreamConfig) Stdout() *broadcaster.Unbuffered {
	return streamConfig.stdout
}

// Stderr returns the standard error in the configuration.
func (streamConfig *StreamConfig) Stderr() *broadcaster.Unbuffered {
	return streamConfig.stderr
}

// Stdin returns the standard input in the configuration.
func (streamConfig *StreamConfig) Stdin() io.ReadCloser {
	return streamConfig.stdin
}

// StdinPipe returns an input writer pipe as an io.WriteCloser.
func (streamConfig *StreamConfig) StdinPipe() io.WriteCloser {
	return streamConfig.stdinPipe
}

// StdoutPipe creates a new io.ReadCloser with an empty bytes pipe.
// It adds this new out pipe to the Stdout broadcaster.
func (streamConfig *StreamConfig) StdoutPipe() io.ReadCloser {
	bytesPipe := ioutils.NewBytesPipe()
	streamConfig.stdout.Add(bytesPipe)
	return bytesPipe
}

// StderrPipe creates a new io.ReadCloser with an empty bytes pipe.
// It adds this new err pipe to the Stderr broadcaster.
func (streamConfig *StreamConfig) StderrPipe() io.ReadCloser {
	bytesPipe := ioutils.NewBytesPipe()
	streamConfig.stderr.Add(bytesPipe)
	return bytesPipe
}

// NewInputPipes creates new pipes for both standard inputs, Stdin and StdinPipe.
func (streamConfig *StreamConfig) NewInputPipes() {
	streamConfig.stdin, streamConfig.stdinPipe = io.Pipe()
}

// NewNopInputPipe creates a new input pipe that will silently drop all messages in the input.
func (streamConfig *StreamConfig) NewNopInputPipe() {
	streamConfig.stdinPipe = ioutils.NopWriteCloser(ioutil.Discard)
}

// CloseStreams ensures that the configured streams are properly closed.
func (streamConfig *StreamConfig) CloseStreams() error {
	var errors []string

	if streamConfig.stdin != nil {
		if err := streamConfig.stdin.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("error close stdin: %s", err))
		}
	}

	if err := streamConfig.stdout.Clean(); err != nil {
		errors = append(errors, fmt.Sprintf("error close stdout: %s", err))
	}

	if err := streamConfig.stderr.Clean(); err != nil {
		errors = append(errors, fmt.Sprintf("error close stderr: %s", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}

	return nil
}
