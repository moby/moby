//go:build windows && !no_embedded_containerd

package embedded

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDefaultAddressUsesStateDir(t *testing.T) {
	stateDir := `C:\ProgramData\docker\execroot\daemon-1\containerd`

	address := defaultAddress(stateDir)
	assert.Check(t, is.Equal(address, defaultAddress(stateDir)))
	assert.Check(t, strings.HasPrefix(address, `\\.\pipe\docker-containerd-embedded-`))
	assert.Check(t, address != defaultAddress(`C:\ProgramData\docker\execroot\daemon-2\containerd`))
}
