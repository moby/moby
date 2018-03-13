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
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
)

func TestResize(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerResize(ctx, cID, types.ResizeOptions{
		Height: 40,
		Width:  40,
	})
	assert.NilError(t, err)
}

func TestResizeWithInvalidSize(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	endpoint := "/containers/" + cID + "/resize?h=foo&w=bar"
	res, _, err := req.Post(endpoint)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(http.StatusBadRequest, res.StatusCode))
}

func TestResizeWhenContainerNotStarted(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("echo"))

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerResize(ctx, cID, types.ResizeOptions{
		Height: 40,
		Width:  40,
	})
	testutil.ErrorContains(t, err, "is not running")
}
