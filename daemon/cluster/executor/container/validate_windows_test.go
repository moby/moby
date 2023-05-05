//go:build windows

package container // import "github.com/docker/docker/daemon/cluster/executor/container"
import (
	"strings"
	"testing"

	"github.com/moby/swarmkit/v2/api"
)

const (
	testAbsPath        = `c:\foo`
	testAbsNonExistent = `c:\some-non-existing-host-path\`
)

func TestControllerValidateMountNamedPipe(t *testing.T) {
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeNamedPipe,
		Source: "",
		Target: `\\.\pipe\foo`,
	}); err == nil || !strings.Contains(err.Error(), "invalid npipe source, source must not be empty") {
		t.Fatalf("expected error, got: %v", err)
	}
}
