package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/environment"
	"github.com/stretchr/testify/require"
)

var testEnv *environment.Execution

func TestMain(m *testing.M) {
	var err error
	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = environment.EnsureFrozenImagesLinux(testEnv)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testEnv.Print()
	os.Exit(m.Run())
}

func setupTest(t *testing.T) func() {
	environment.ProtectAll(t, testEnv)
	return func() { testEnv.Clean(t) }
}

type containerConstructor func(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig)

func createSimpleContainer(ctx context.Context, t *testing.T, client client.APIClient, name string, f ...containerConstructor) string {
	config := &container.Config{
		Cmd:   []string{"top"},
		Image: "busybox",
	}
	hostConfig := &container.HostConfig{}
	networkingConfig := &network.NetworkingConfig{}

	for _, fn := range f {
		fn(config, hostConfig, networkingConfig)
	}

	c, err := client.ContainerCreate(ctx, config, hostConfig, networkingConfig, name)
	require.NoError(t, err)

	return c.ID
}

func runSimpleContainer(ctx context.Context, t *testing.T, client client.APIClient, name string, f ...containerConstructor) string {
	cID := createSimpleContainer(ctx, t, client, name, f...)

	err := client.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	require.NoError(t, err)

	return cID
}
