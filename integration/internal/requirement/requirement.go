package requirement // import "github.com/docker/docker/integration/internal/requirement"

import (
	"net/http"
	"strings"
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
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		t.Fatalf("Timeout for GET request on %s", url)
	}
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}
