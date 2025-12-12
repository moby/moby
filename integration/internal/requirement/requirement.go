package requirement

import (
	"errors"
	"net"
	"net/http"
	"path"
	"reflect"
	"runtime"
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
	if errors.Is(err, net.ErrClosed) {
		t.Fatalf("Timeout for GET request on %s", url)
	}
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}

// TestRequires checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func TestRequires(t *testing.T, requirements ...func() bool) {
	t.Helper()
	for _, check := range requirements {
		if !check() {
			requirementFunc := runtime.FuncForPC(reflect.ValueOf(check).Pointer()).Name()
			_, req, _ := strings.Cut(path.Base(requirementFunc), ".")
			t.Skipf("unmatched requirement %s", req)
		}
	}
}
