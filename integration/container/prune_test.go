package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

func TestPruneSince(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	client := testEnv.APIClient()

	cID1 := container.Run(ctx, t, client, container.WithCmd("true"))
	poll.WaitOn(t, container.IsInState(ctx, client, cID1, "exited"), poll.WithDelay(100*time.Millisecond))

	since := request.DaemonUnixTime(ctx, t, client, testEnv)

	cID2 := container.Run(ctx, t, client, container.WithCmd("true"))
	poll.WaitOn(t, container.IsInState(ctx, client, cID2, "exited"), poll.WithDelay(100*time.Millisecond))

	args := filters.NewArgs()
	args.UnmarshalJSON([]byte(fmt.Sprintf(`{"since": {"%s": true}}`, since)))
	report, _ := client.ContainersPrune(ctx, args)
	assert.Assert(t, reflect.DeepEqual(report.ContainersDeleted, []string{cID2}))
}
