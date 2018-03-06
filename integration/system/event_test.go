package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/strslice"
	req "github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/stretchr/testify/assert"
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

// Test case for #18888: Events messages have been switched from generic
// `JSONMessage` to `events.Message` types. The switch does not break the
// backward compatibility so old `JSONMessage` could still be used.
// This test verifies that backward compatibility maintains.
func TestEventsBackwardsCompatible(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	since := request.DaemonTime(ctx, t, client, testEnv)
	ts := strconv.FormatInt(since.Unix(), 10)

	cID := container.Create(t, ctx, client)

	// In case there is no events, the API should have responded immediately (not blocking),
	// The test here makes sure the response time is less than 3 sec.
	expectedTime := time.Now().Add(3 * time.Second)
	emptyResp, emptyBody, err := req.Get("/events")
	require.NoError(t, err)
	defer emptyBody.Close()
	assert.Equal(t, http.StatusOK, emptyResp.StatusCode)
	assert.True(t, time.Now().Before(expectedTime), "timeout waiting for events api to respond, should have responded immediately")

	// We also test to make sure the `events.Message` is compatible with `JSONMessage`
	q := url.Values{}
	q.Set("since", ts)
	_, body, err := req.Get("/events?" + q.Encode())
	require.NoError(t, err)
	defer body.Close()

	dec := json.NewDecoder(body)
	var containerCreateEvent *jsonmessage.JSONMessage
	for {
		var event jsonmessage.JSONMessage
		if err := dec.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if event.Status == "create" && event.ID == cID {
			containerCreateEvent = &event
			break
		}
	}

	assert.NotNil(t, containerCreateEvent)
	assert.Equal(t, "create", containerCreateEvent.Status)
	assert.Equal(t, cID, containerCreateEvent.ID)
	assert.Equal(t, "busybox", containerCreateEvent.From)
}
