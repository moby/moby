package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	"golang.org/x/net/context"
)

// ContainerWait waits for the container designated by containerID to meet
// given condition, either "not-running" (default), "next-exit", or "removed".
// This is a blocking function.
//
// For an API with version 1.30+, if a non-nil ackChan is passed, it will be closed
// as soon as the server acknowledges it has successfully started waiting for the
// container (honoring the given condition), by writing the HTTP header early.
// This allows the caller to synchronize ContainerWait with other calls.
//
// For example, assuming the container identified by containerID is stopped,
//
//	ackChan := make(chan struct{})
//	go func() {
//		select {
//		case <-ackChan:
//			cli.ContainerStart(ctx, containerID, options)
//		case <-ctx.Done():
//		}
//	}()
// 	result, err := cli.ContainerWait(ctx, containerID, container.WaitConditionNextExit, ackChan)
//
// this would allow the caller to avoid a race condition where the container
// starts but exits before the caller is able to wait on it, resulting in an
// indefinite wait.
//
// If this client's API version is strictly less than 1.30, condition is
// ignored and ContainerWait will wait for the container to not be running.
func (cli *Client) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition, ackChan chan<- struct{}) (container.ContainerWaitOKBody, error) {
	query := url.Values{}

	if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.30") {
		query.Set("condition", string(condition))
	}

	var result container.ContainerWaitOKBody

	resp, err := cli.post(ctx, "/containers/"+containerID+"/wait", query, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return result, err
	}
	if ackChan != nil {
		close(ackChan)
	}

	err = json.NewDecoder(resp.body).Decode(&result)
	return result, err
}
