package request

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/stretchr/testify/require"
)

// NewAPIClient returns a docker API client configured from environment variables
func NewAPIClient(t *testing.T) client.APIClient {
	clt, err := client.NewEnvClient()
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
	verStr := api.DefaultVersion
	customHeaders := map[string]string{}
	return client.NewClient(host, verStr, httpClient, customHeaders)
}
