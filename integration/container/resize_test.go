package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	req "github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResize(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerResize(ctx, cID, types.ResizeOptions{
		Height: 40,
		Width:  40,
	})
	require.NoError(t, err)
}

func TestResizeWithInvalidSize(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	endpoint := "/containers/" + cID + "/resize?h=foo&w=bar"
	res, _, err := req.Post(endpoint)
	require.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusBadRequest)
}

func TestResizeWhenContainerNotStarted(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("echo"))

	poll.WaitOn(t, containerIsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerResize(ctx, cID, types.ResizeOptions{
		Height: 40,
		Width:  40,
	})
	testutil.ErrorContains(t, err, "is not running")
}
