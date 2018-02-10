package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinksEtcHostsContentMatch(t *testing.T) {
	skip.If(t, !testEnv.IsLocalDaemon())

	hosts, err := ioutil.ReadFile("/etc/hosts")
	skip.If(t, os.IsNotExist(err))

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"cat", "/etc/hosts"},
		},
		&container.HostConfig{
			NetworkMode: "host",
		},
		nil,
		"")
	require.NoError(t, err)

	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, containerIsStopped(ctx, client, c.ID), poll.WithDelay(100*time.Millisecond))

	body, err := client.ContainerLogs(ctx, c.ID, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	require.NoError(t, err)
	defer body.Close()

	var b bytes.Buffer
	_, err = stdcopy.StdCopy(&b, ioutil.Discard, body)
	require.NoError(t, err)

	assert.Equal(t, string(hosts), b.String())
}
