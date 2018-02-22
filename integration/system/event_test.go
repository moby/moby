package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"testing"

	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/require"
)

func TestEvents(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client)

	id, err := client.ContainerExecCreate(ctx, cID,
		types.ExecConfig{
			Cmd: strslice.StrSlice([]string{"echo", "hello"}),
		},
	)
	require.NoError(t, err)

	filters := filters.NewArgs(
		filters.Arg("container", cID),
		filters.Arg("event", "exec_die"),
	)
	msg, errors := client.Events(ctx, types.EventsOptions{
		Filters: filters,
	})

	err = client.ContainerExecStart(ctx, id.ID,
		types.ExecStartCheck{
			Detach: true,
			Tty:    false,
		},
	)
	require.NoError(t, err)

	select {
	case m := <-msg:
		require.Equal(t, m.Type, "container")
		require.Equal(t, m.Actor.ID, cID)
		require.Equal(t, m.Action, "exec_die")
		require.Equal(t, m.Actor.Attributes["execID"], id.ID)
		require.Equal(t, m.Actor.Attributes["exitCode"], "0")
	case err = <-errors:
		t.Fatal(err)
	case <-time.After(time.Second * 3):
		t.Fatal("timeout hit")
	}

}
