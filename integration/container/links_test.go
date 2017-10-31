package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/request"
	"github.com/docker/docker/internal/testutil"
)

func runContainerLinks(ctx context.Context, t *testing.T, client client.APIClient, cntCfg *container.Config, hstCfg *container.HostConfig, nwkCfg *network.NetworkingConfig, cntName string) (string, error) {
	cnt, err := client.ContainerCreate(ctx, cntCfg, hstCfg, nwkCfg, cntName)
	if err != nil {
		return "", err
	}

	err = client.ContainerStart(ctx, cnt.ID, types.ContainerStartOptions{})
	return cnt.ID, err
}

func TestLinkedContainerAliasWithSeparator(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cntConfig := &container.Config{
		Image: "busybox",
		Tty:   true,
		Cmd:   strslice.StrSlice([]string{"top"}),
	}

	var (
		err error
	)

	runContainer(ctx, t, client,
		cntConfig,
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"a0",
	)

	_, err = runContainerLinks(ctx, t, client,
		cntConfig,
		&container.HostConfig{
			Links: []string{"a0:links/sep"},
		},
		&network.NetworkingConfig{},
		"b0",
	)
	testutil.ErrorContains(t, err, "Invalid alias name")
}
