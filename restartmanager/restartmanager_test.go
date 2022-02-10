package restartmanager // import "github.com/docker/docker/restartmanager"

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

func TestRestartManagerTimeout(t *testing.T) {
	rm := New(container.RestartPolicy{Name: "always"}, 0)
	var duration = 1 * time.Second
	should, _, err := rm.ShouldRestart(0, false, duration)
	if err != nil {
		t.Fatal(err)
	}
	if !should {
		t.Fatal("container should be restarted")
	}
	if rm.timeout != defaultTimeout {
		t.Fatalf("restart manager should have a timeout of 100 ms but has %s", rm.timeout)
	}
}

func TestRestartManagerTimeoutReset(t *testing.T) {
	rm := New(container.RestartPolicy{Name: "always"}, 0)
	rm.timeout = 5 * time.Second
	var duration = 10 * time.Second
	_, _, err := rm.ShouldRestart(0, false, duration)
	if err != nil {
		t.Fatal(err)
	}
	if rm.timeout != defaultTimeout {
		t.Fatalf("restart manager should have a timeout of 100 ms but has %s", rm.timeout)
	}
}
