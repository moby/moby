package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinksEtcHostsContentMatch(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon())

	hosts, err := ioutil.ReadFile("/etc/hosts")
	skip.If(t, os.IsNotExist(err))

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithNetworkMode("host"))
	res, err := container.Exec(ctx, client, cID, []string{"cat", "/etc/hosts"})
	require.NoError(t, err)
	require.Empty(t, res.Stderr())
	require.Equal(t, 0, res.ExitCode)

	assert.Equal(t, string(hosts), res.Stdout())
}

func TestLinksContainerNames(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	container.Run(t, ctx, client, container.WithName("first"))
	container.Run(t, ctx, client, container.WithName("second"), container.WithLinks("first:first"))

	f := filters.NewArgs(filters.Arg("name", "first"))

	containers, err := client.ContainerList(ctx, types.ContainerListOptions{
		Filters: f,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(containers))
	assert.Equal(t, []string{"/first", "/second/first"}, containers[0].Names)
}
