package signal // import "github.com/docker/docker/pkg/signal"

import (
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseSignal(t *testing.T) {
	_, checkAtoiError := ParseSignal("0")
	assert.Check(t, is.Error(checkAtoiError, "Invalid signal: 0"))

	_, error := ParseSignal("SIG")
	assert.Check(t, is.Error(error, "Invalid signal: SIG"))

	for sigStr := range SignalMap {
		responseSignal, error := ParseSignal(sigStr)
		assert.Check(t, error)
		signal := SignalMap[sigStr]
		assert.Check(t, is.DeepEqual(signal, responseSignal))
	}
}

func TestValidSignalForPlatform(t *testing.T) {
	isValidSignal := ValidSignalForPlatform(syscall.Signal(0))
	assert.Check(t, is.Equal(false, isValidSignal))

	for _, sigN := range SignalMap {
		isValidSignal = ValidSignalForPlatform(sigN)
		assert.Check(t, is.Equal(true, isValidSignal))
	}
}
