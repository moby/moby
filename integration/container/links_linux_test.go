package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
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

	cID := container.Run(t, ctx, client, container.WithCmd("cat", "/etc/hosts"), container.WithNetworkMode("host"))

	poll.WaitOn(t, containerIsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	body, err := client.ContainerLogs(ctx, cID, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	require.NoError(t, err)
	defer body.Close()

	var b bytes.Buffer
	_, err = stdcopy.StdCopy(&b, ioutil.Discard, body)
	require.NoError(t, err)

	assert.Equal(t, string(hosts), b.String())
}
