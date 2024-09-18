package nat // import "github.com/docker/docker/integration/network/nat"

import (
	"testing"

	"github.com/docker/docker/integration/internal/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWindowsNoDisableIPv4(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	_, err := network.Create(ctx, c, "ipv6only",
		network.WithDriver("nat"),
		network.WithIPv4(false),
	)
	// This error message should change to "IPv4 cannot be disabled on Windows"
	// when "--experimental" is no longer required to disable IPv4. But, there's
	// no way to start a second daemon with "--experimental" in Windows CI.
	assert.Check(t, is.ErrorContains(err,
		"IPv4 can only be disabled if experimental features are enabled"))
}
