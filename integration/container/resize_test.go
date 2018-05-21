package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	req "github.com/docker/docker/internal/test/request"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
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
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.32"), "broken in earlier versions")
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
	assert.Check(t, is.ErrorContains(err, "is not running"))
}
