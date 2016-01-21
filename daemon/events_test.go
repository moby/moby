package daemon

import (
	"testing"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/events"
	containertypes "github.com/docker/engine-api/types/container"
)

func TestLogContainerCopyLabels(t *testing.T) {
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
	daemon.LogContainerEvent(container, "create")

	if _, mutated := container.Config.Labels["image"]; mutated {
		t.Fatalf("Expected to not mutate the container labels, got %q", container.Config.Labels)
	}
}
