package client

import (
	"context"
	"net/http"
	"net/url"
)

// ContainerAttachOptions holds parameters to attach to a container.
type ContainerAttachOptions struct {
	Stream     bool
	Stdin      bool
	Stdout     bool
	Stderr     bool
	DetachKeys string
	Logs       bool
}

// ContainerAttachResult is the result from attaching to a container.
type ContainerAttachResult struct {
	HijackedResponse
}

// ContainerAttach attaches a connection to a container in the server.
// It returns a [HijackedResponse] with the hijacked connection
// and a reader to get output. It's up to the called to close
// the hijacked connection by calling [HijackedResponse.Close].
//
// The stream format on the response uses one of two formats:
//
//   - If the container is using a TTY, there is only a single stream (stdout)
//     and data is copied directly from the container output stream, no extra
//     multiplexing or headers.
//   - If the container is *not* using a TTY, streams for stdout and stderr are
//     multiplexed.
//
// The format of the multiplexed stream is defined in the [stdcopy] package,
// and as follows:
//
//	[8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}[]byte{OUTPUT}
//
// STREAM_TYPE can be 1 for [Stdout] and 2 for [Stderr]. Refer to [stdcopy.StdType]
// for details. SIZE1, SIZE2, SIZE3, and SIZE4 are four bytes of uint32 encoded
// as big endian, this is the size of OUTPUT. You can use [stdcopy.StdCopy]
// to demultiplex this stream.
//
// [stdcopy]: https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy
// [stdcopy.StdCopy]: https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy#StdCopy
// [stdcopy.StdType]: https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy#StdType
// [Stdout]: https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy#Stdout
// [Stderr]: https://pkg.go.dev/github.com/moby/moby/api/pkg/stdcopy#Stderr
func (cli *Client) ContainerAttach(ctx context.Context, containerID string, options ContainerAttachOptions) (ContainerAttachResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerAttachResult{}, err
	}

	query := url.Values{}
	if options.Stream {
		query.Set("stream", "1")
	}
	if options.Stdin {
		query.Set("stdin", "1")
	}
	if options.Stdout {
		query.Set("stdout", "1")
	}
	if options.Stderr {
		query.Set("stderr", "1")
	}
	if options.DetachKeys != "" {
		query.Set("detachKeys", options.DetachKeys)
	}
	if options.Logs {
		query.Set("logs", "1")
	}

	hijacked, err := cli.postHijacked(ctx, "/containers/"+containerID+"/attach", query, nil, http.Header{
		"Content-Type": {"text/plain"},
	})
	if err != nil {
		return ContainerAttachResult{}, err
	}

	return ContainerAttachResult{HijackedResponse: hijacked}, nil
}
