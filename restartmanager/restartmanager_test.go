package restartmanager

import (
	"testing"
	"time"

	"github.com/docker/engine-api/types/container"
)

func TestRestartManagerTimeout(t *testing.T) {
	rm := New(container.RestartPolicy{Name: "always"}, 0).(*restartManager)
	should, _, err := rm.ShouldRestart(0, false, 1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !should {
		t.Fatal("container should be restarted")
	}
	if rm.timeout != 100*time.Millisecond {
		t.Fatalf("restart manager should have a timeout of 100ms but has %s", rm.timeout)
	}
}

func TestRestartManagerTimeoutReset(t *testing.T) {
	rm := New(container.RestartPolicy{Name: "always"}, 0).(*restartManager)
	rm.timeout = 5 * time.Second
	_, _, err := rm.ShouldRestart(0, false, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rm.timeout != 100*time.Millisecond {
		t.Fatalf("restart manager should have a timeout of 100ms but has %s", rm.timeout)
	}
}
