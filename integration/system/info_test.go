package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestInfoAPI(t *testing.T) {
	client := request.NewAPIClient(t)

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"ContainersRunning",
		"ContainersPaused",
		"ContainersStopped",
		"Images",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"OSType",
		"Architecture",
		"MemTotal",
		"KernelVersion",
		"Driver",
		"ServerVersion",
		"SecurityOptions"}

	out := fmt.Sprintf("%+v", info)
	for _, linePrefix := range stringsToCheck {
		assert.Check(t, is.Contains(out, linePrefix))
	}
}
