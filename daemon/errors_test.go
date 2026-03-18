package daemon

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestContainerNotRunningError(t *testing.T) {
	err := errNotRunning("12345")
	assert.Check(t, isNotRunning(err))
}
