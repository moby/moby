//go:build darwin || linux
// +build darwin linux

package signal // import "github.com/docker/docker/pkg/signal"

import (
	"os"
	"syscall"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCatchAll(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	CatchAll(sigs)
	defer StopCatch(sigs)

	listOfSignals := map[string]string{
		"CONT": syscall.SIGCONT.String(),
		"HUP":  syscall.SIGHUP.String(),
		"CHLD": syscall.SIGCHLD.String(),
		"ILL":  syscall.SIGILL.String(),
		"FPE":  syscall.SIGFPE.String(),
		"CLD":  syscall.SIGCLD.String(),
	}

	for sigStr := range listOfSignals {
		if signal, ok := SignalMap[sigStr]; ok {
			syscall.Kill(syscall.Getpid(), signal)
			s := <-sigs
			assert.Check(t, is.Equal(s.String(), signal.String()))
		}
	}
}

func TestCatchAllIgnoreSigUrg(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	CatchAll(sigs)
	defer StopCatch(sigs)

	err := syscall.Kill(syscall.Getpid(), syscall.SIGURG)
	assert.NilError(t, err)
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case s := <-sigs:
		t.Fatalf("expected no signals to be handled, but received %q", s.String())
	}
}

func TestStopCatch(t *testing.T) {
	signal := SignalMap["HUP"]
	channel := make(chan os.Signal, 1)
	CatchAll(channel)
	syscall.Kill(syscall.Getpid(), signal)
	signalString := <-channel
	assert.Check(t, is.Equal(signalString.String(), signal.String()))

	StopCatch(channel)
	_, ok := <-channel
	assert.Check(t, is.Equal(ok, false))
}
