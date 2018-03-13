package request // import "github.com/docker/docker/integration/internal/request"

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/environment"
	"github.com/gotestyourself/gotestyourself/assert"
)

// NewAPIClient returns a docker API client configured from environment variables
func NewAPIClient(t *testing.T, ops ...func(*client.Client) error) client.APIClient {
	ops = append([]func(*client.Client) error{client.FromEnv}, ops...)
	clt, err := client.NewClientWithOpts(ops...)
	assert.NilError(t, err)
	return clt
}

// DaemonTime provides the current time on the daemon host
func DaemonTime(ctx context.Context, t *testing.T, client client.APIClient, testEnv *environment.Execution) time.Time {
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	assert.NilError(t, err, "invalid time format in GET /info response")
	return dt
}

// DaemonUnixTime returns the current time on the daemon host with nanoseconds precision.
// It return the time formatted how the client sends timestamps to the server.
func DaemonUnixTime(ctx context.Context, t *testing.T, client client.APIClient, testEnv *environment.Execution) string {
	dt := DaemonTime(ctx, t, client, testEnv)
	return fmt.Sprintf("%d.%09d", dt.Unix(), int64(dt.Nanosecond()))
}
