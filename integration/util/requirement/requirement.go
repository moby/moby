package requirement

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// HasHubConnectivity checks to see if https://hub.docker.com is
// accessible from the present environment
func HasHubConnectivity(t *testing.T) bool {
	// Set a timeout on the GET at 15s
	var timeout = 15 * time.Second
	var url = "https://hub.docker.com"

	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		t.Fatalf("Timeout for GET request on %s", url)
	}
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}
