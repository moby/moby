package container

import (
	"testing"

	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types/container"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestContainerStopSignal(c *check.C) {
	co := &Container{
		CommonContainer: CommonContainer{
			Config: &container.Config{},
		},
	}

	def, err := signal.ParseSignal(signal.DefaultStopSignal)
	if err != nil {
		c.Fatal(err)
	}

	ss := co.StopSignal()
	if ss != int(def) {
		c.Fatalf("Expected %v, got %v", def, ss)
	}

	co = &Container{
		CommonContainer: CommonContainer{
			Config: &container.Config{StopSignal: "SIGKILL"},
		},
	}
	ss = co.StopSignal()
	if ss != 9 {
		c.Fatalf("Expected 9, got %v", ss)
	}
}
