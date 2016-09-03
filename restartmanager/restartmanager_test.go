package restartmanager

import (
	"testing"
	"time"

	"github.com/docker/engine-api/types/container"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestRestartManagerTimeout(c *check.C) {
	rm := New(container.RestartPolicy{Name: "always"}, 0).(*restartManager)
	should, _, err := rm.ShouldRestart(0, false, 1*time.Second)
	if err != nil {
		c.Fatal(err)
	}
	if !should {
		c.Fatal("container should be restarted")
	}
	if rm.timeout != 100*time.Millisecond {
		c.Fatalf("restart manager should have a timeout of 100ms but has %s", rm.timeout)
	}
}

func (s *DockerSuite) TestRestartManagerTimeoutReset(c *check.C) {
	rm := New(container.RestartPolicy{Name: "always"}, 0).(*restartManager)
	rm.timeout = 5 * time.Second
	_, _, err := rm.ShouldRestart(0, false, 10*time.Second)
	if err != nil {
		c.Fatal(err)
	}
	if rm.timeout != 100*time.Millisecond {
		c.Fatalf("restart manager should have a timeout of 100ms but has %s", rm.timeout)
	}
}
