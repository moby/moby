package requirement // import "github.com/docker/docker/integration/internal/requirement"

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/gotestyourself/gotestyourself/icmd"
)

// HasHubConnectivity checks to see if https://hub.docker.com is
// accessible from the present environment
func HasHubConnectivity(t *testing.T) bool {
	t.Helper()
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

func overlayFSSupported() bool {
	result := icmd.RunCommand("/bin/sh", "-c", "cat /proc/filesystems")
	if result.Error != nil {
		return false
	}
	return strings.Contains(result.Combined(), "overlay\n")
}

// Overlay2Supported returns true if the current system supports overlay2 as graphdriver
func Overlay2Supported(kernelVersion string) bool {
	if !overlayFSSupported() {
		return false
	}

	daemonV, err := kernel.ParseRelease(kernelVersion)
	if err != nil {
		return false
	}
	requiredV := kernel.VersionInfo{Kernel: 4}
	return kernel.CompareKernelVersion(*daemonV, requiredV) > -1

}
