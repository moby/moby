package requirement

import (
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// HasHubConnectivity checks to see if https://hub.docker.com is
// accessible from the present environment
func HasHubConnectivity(t *testing.T) bool {
	t.Helper()
	// Set a timeout on the GET at 15s
	timeout := 15 * time.Second
	url := "https://hub.docker.com"

	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if errors.Is(err, net.ErrClosed) {
		t.Fatalf("Timeout for GET request on %s", url)
	}
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}
