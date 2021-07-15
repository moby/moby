package signal // import "github.com/docker/docker/pkg/signal"

import (
	"syscall"
	"testing"
)

func TestParseSignal(t *testing.T) {
	_, err := ParseSignal("0")
	expectedErr := "Invalid signal: 0"
	if err == nil || err.Error() != expectedErr {
		t.Errorf("expected  %q, but got %v", expectedErr, err)
	}

	_, err = ParseSignal("SIG")
	expectedErr = "Invalid signal: SIG"
	if err == nil || err.Error() != expectedErr {
		t.Errorf("expected  %q, but got %v", expectedErr, err)
	}

	for sigStr := range SignalMap {
		responseSignal, err := ParseSignal(sigStr)
		if err != nil {
			t.Error(err)
		}
		signal := SignalMap[sigStr]
		if responseSignal != signal {
			t.Errorf("expected: %q, got: %q", signal, responseSignal)
		}
	}
}

func TestValidSignalForPlatform(t *testing.T) {
	isValidSignal := ValidSignalForPlatform(syscall.Signal(0))
	if isValidSignal {
		t.Error("expected !isValidSignal")
	}

	for _, sigN := range SignalMap {
		isValidSignal = ValidSignalForPlatform(sigN)
		if !isValidSignal {
			t.Error("expected isValidSignal")
		}
	}
}
