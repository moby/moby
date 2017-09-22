package system

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/environment"
	"github.com/stretchr/testify/require"
)

// Time provides the current time on the daemon host
func Time(t *testing.T, client client.APIClient, testEnv *environment.Execution) time.Time {
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}

	ctx := context.Background()
	info, err := client.Info(ctx)
	require.Nil(t, err)

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	require.Nil(t, err, "invalid time format in GET /info response")
	return dt
}

// Version provides the version of the daemon
func Version(client client.APIClient) (types.Version, error) {
	ctx := context.Background()
	return client.ServerVersion(ctx)
}

// EventsSince returns event and error streams since a provided time
func EventsSince(client client.APIClient, since string) (<-chan events.Message, <-chan error, func()) {
	eventOptions := types.EventsOptions{
		Since: since,
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, errs := client.Events(ctx, eventOptions)

	return events, errs, cancel
}
