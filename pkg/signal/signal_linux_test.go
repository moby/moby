// +build darwin linux

package signal // import "github.com/docker/docker/pkg/signal"

import (
	"os"
	"syscall"
	"testing"
	"time"
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
			_ = syscall.Kill(syscall.Getpid(), signal)
			s := <-sigs
			if s.String() != signal.String() {
				t.Errorf("expected: %q, got: %q", signal, s)
			}
		}
	}
}

func TestCatchAllIgnoreSigUrg(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	CatchAll(sigs)
	defer StopCatch(sigs)

	err := syscall.Kill(syscall.Getpid(), syscall.SIGURG)
	if err != nil {
		t.Fatal(err)
	}
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
	_ = syscall.Kill(syscall.Getpid(), signal)
	signalString := <-channel
	if signalString.String() != signal.String() {
		t.Errorf("expected: %q, got: %q", signal, signalString)
	}

	StopCatch(channel)
	_, ok := <-channel
	if ok {
		t.Error("expected: !ok, got: ok")
	}
}
