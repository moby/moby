package container

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"gotest.tools/v3/poll"
)

// logsContains verifies the container contains the given text in stdout.
func logsContains(ctx context.Context, apiClient client.APIClient, containerID string, logString string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		logs, err := apiClient.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
			ShowStdout: true,
		})
		if err != nil {
			return poll.Error(err)
		}
		defer logs.Close()

		var stdout bytes.Buffer
		_, err = stdcopy.StdCopy(&stdout, io.Discard, logs)
		if err != nil {
			return poll.Error(err)
		}
		if strings.Contains(stdout.String(), logString) {
			return poll.Success()
		}
		return poll.Continue("waiting for logstring '%s' in container", logString)
	}
}
