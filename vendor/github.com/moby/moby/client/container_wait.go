package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"

	"github.com/moby/moby/api/types/container"
)

const containerWaitErrorMsgLimit = 2 * 1024 /* Max: 2KiB */

// ContainerWaitOptions holds options for [Client.ContainerWait].
type ContainerWaitOptions struct {
	Condition container.WaitCondition
}

// ContainerWaitResult defines the result from the [Client.ContainerWait] method.
type ContainerWaitResult struct {
	Result <-chan container.WaitResponse
	Error  <-chan error
}

// ContainerWait waits until the specified container is in a certain state
// indicated by the given condition, either;
//
//   - "not-running" ([container.WaitConditionNotRunning]) (default)
//   - "next-exit" ([container.WaitConditionNextExit])
//   - "removed" ([container.WaitConditionRemoved])
//
// ContainerWait blocks until the request has been acknowledged by the server
// (with a response header), then returns two channels on which the caller can
// wait for the exit status of the container or an error if there was a problem
// either beginning the wait request or in getting the response. This allows the
// caller to synchronize ContainerWait with other calls, such as specifying a
// "next-exit" condition ([container.WaitConditionNextExit]) before issuing a
// [Client.ContainerStart] request.
func (cli *Client) ContainerWait(ctx context.Context, containerID string, options ContainerWaitOptions) ContainerWaitResult {
	resultC := make(chan container.WaitResponse)
	errC := make(chan error, 1)

	containerID, err := trimID("container", containerID)
	if err != nil {
		errC <- err
		return ContainerWaitResult{Result: resultC, Error: errC}
	}

	query := url.Values{}
	if options.Condition != "" {
		query.Set("condition", string(options.Condition))
	}

	resp, err := cli.post(ctx, "/containers/"+containerID+"/wait", query, nil, nil)
	if err != nil {
		defer ensureReaderClosed(resp)
		errC <- err
		return ContainerWaitResult{Result: resultC, Error: errC}
	}

	go func() {
		defer ensureReaderClosed(resp)

		responseText := bytes.NewBuffer(nil)
		stream := io.TeeReader(resp.Body, responseText)

		var res container.WaitResponse
		if err := json.NewDecoder(stream).Decode(&res); err != nil {
			// NOTE(nicks): The /wait API does not work well with HTTP proxies.
			// At any time, the proxy could cut off the response stream.
			//
			// But because the HTTP status has already been written, the proxy's
			// only option is to write a plaintext error message.
			//
			// If there's a JSON parsing error, read the real error message
			// off the body and send it to the client.
			if errors.As(err, new(*json.SyntaxError)) {
				_, _ = io.ReadAll(io.LimitReader(stream, containerWaitErrorMsgLimit))
				errC <- errors.New(responseText.String())
			} else {
				errC <- err
			}
			return
		}

		resultC <- res
	}()

	return ContainerWaitResult{Result: resultC, Error: errC}
}
