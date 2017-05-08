// +build windows

package runconfig

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestValidatePrivileged(t *testing.T) {
	expected := "invalid --privileged: Windows does not support this feature"
	err := validatePrivileged(&container.HostConfig{Privileged: true})
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected %s", expected)
	}
}
