package system

import (
	"fmt"
	"testing"

	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestInfoAPI(t *testing.T) {
	client := request.NewAPIClient(t)

	info, err := client.Info(context.Background())
	require.NoError(t, err)

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
		assert.Contains(t, out, linePrefix)
	}
}
