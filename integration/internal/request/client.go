package request // import "github.com/docker/docker/integration/internal/request"

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/environment"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/stretchr/testify/require"
)

// NewAPIClient returns a docker API client configured from environment variables
func NewAPIClient(t *testing.T, ops ...func(*client.Client) error) client.APIClient {
	ops = append([]func(*client.Client) error{client.FromEnv}, ops...)
	clt, err := client.NewClientWithOpts(ops...)
	require.NoError(t, err)
	return clt
}

// NewTLSAPIClient returns a docker API client configured with the
// provided TLS settings
func NewTLSAPIClient(t *testing.T, host, cacertPath, certPath, keyPath string) (client.APIClient, error) {
	opts := tlsconfig.Options{
		CAFile:             cacertPath,
		CertFile:           certPath,
		KeyFile:            keyPath,
		ExclusiveRootPools: true,
	}
	config, err := tlsconfig.Client(opts)
	require.Nil(t, err)
	tr := &http.Transport{
		TLSClientConfig: config,
		DialContext: (&net.Dialer{
			KeepAlive: 30 * time.Second,
			Timeout:   30 * time.Second,
		}).DialContext,
	}
	proto, addr, _, err := client.ParseHost(host)
	require.Nil(t, err)

	sockets.ConfigureTransport(tr, proto, addr)

	httpClient := &http.Client{
		Transport:     tr,
		CheckRedirect: client.CheckRedirect,
	}
	return client.NewClientWithOpts(client.WithHost(host), client.WithHTTPClient(httpClient))
}

// daemonTime provides the current time on the daemon host
func daemonTime(ctx context.Context, t *testing.T, client client.APIClient, testEnv *environment.Execution) time.Time {
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}

	info, err := client.Info(ctx)
	require.NoError(t, err)

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	require.NoError(t, err, "invalid time format in GET /info response")
	return dt
}

// DaemonUnixTime returns the current time on the daemon host with nanoseconds precision.
// It return the time formatted how the client sends timestamps to the server.
func DaemonUnixTime(ctx context.Context, t *testing.T, client client.APIClient, testEnv *environment.Execution) string {
	dt := daemonTime(ctx, t, client, testEnv)
	return fmt.Sprintf("%d.%09d", dt.Unix(), int64(dt.Nanosecond()))
}
