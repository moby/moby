package daemon

import (
	"time"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/events"
	containertypes "github.com/docker/engine-api/types/container"
	eventtypes "github.com/docker/engine-api/types/events"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestLogContainerEventCopyLabels(c *check.C) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	container := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "container_id",
			Name: "container_name",
			Config: &containertypes.Config{
				Image: "image_name",
				Labels: map[string]string{
					"node": "1",
					"os":   "alpine",
				},
			},
		},
	}
	daemon := &Daemon{
		EventsService: e,
	}
	daemon.LogContainerEvent(container, "create")

	if _, mutated := container.Config.Labels["image"]; mutated {
		c.Fatalf("Expected to not mutate the container labels, got %q", container.Config.Labels)
	}

	validateTestAttributes(c, l, map[string]string{
		"node": "1",
		"os":   "alpine",
	})
}

func (s *DockerSuite) TestLogContainerEventWithAttributes(c *check.C) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	container := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "container_id",
			Name: "container_name",
			Config: &containertypes.Config{
				Labels: map[string]string{
					"node": "1",
					"os":   "alpine",
				},
			},
		},
	}
	daemon := &Daemon{
		EventsService: e,
	}
	attributes := map[string]string{
		"node": "2",
		"foo":  "bar",
	}
	daemon.LogContainerEventWithAttributes(container, "create", attributes)

	validateTestAttributes(c, l, map[string]string{
		"node": "1",
		"foo":  "bar",
	})
}

func validateTestAttributes(c *check.C, l chan interface{}, expectedAttributesToTest map[string]string) {
	select {
	case ev := <-l:
		event, ok := ev.(eventtypes.Message)
		if !ok {
			c.Fatalf("Unexpected event message: %q", ev)
		}
		for key, expected := range expectedAttributesToTest {
			actual, ok := event.Actor.Attributes[key]
			if !ok || actual != expected {
				c.Fatalf("Expected value for key %s to be %s, but was %s (event:%v)", key, expected, actual, event)
			}
		}
	case <-time.After(10 * time.Second):
		c.Fatalf("LogEvent test timed out")
	}
}
