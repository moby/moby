package daemon

import (
	"time"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/events"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	eventtypes "github.com/docker/engine-api/types/events"
	"github.com/go-check/check"
)

func reset(c *container.Container) {
	c.State = &container.State{}
	c.State.Health = &container.Health{}
	c.State.Health.Status = types.Starting
}

func (s *DockerSuite) TestNoneHealthcheck(c *check.C) {
	ct := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "container_id",
			Name: "container_name",
			Config: &containertypes.Config{
				Image: "image_name",
				Healthcheck: &containertypes.HealthConfig{
					Test: []string{"NONE"},
				},
			},
			State: &container.State{},
		},
	}
	daemon := &Daemon{}

	daemon.initHealthMonitor(ct)
	if ct.State.Health != nil {
		c.Errorf("Expecting Health to be nil, but was not")
	}
}

func (s *DockerSuite) TestHealthStates(c *check.C) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	expect := func(expected string) {
		select {
		case event := <-l:
			ev := event.(eventtypes.Message)
			if ev.Status != expected {
				c.Errorf("Expecting event %#v, but got %#v\n", expected, ev.Status)
			}
		case <-time.After(1 * time.Second):
			c.Errorf("Expecting event %#v, but got nothing\n", expected)
		}
	}

	ct := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "container_id",
			Name: "container_name",
			Config: &containertypes.Config{
				Image: "image_name",
			},
		},
	}
	daemon := &Daemon{
		EventsService: e,
	}

	ct.Config.Healthcheck = &containertypes.HealthConfig{
		Retries: 1,
	}

	reset(ct)

	handleResult := func(startTime time.Time, exitCode int) {
		handleProbeResult(daemon, ct, &types.HealthcheckResult{
			Start:    startTime,
			End:      startTime,
			ExitCode: exitCode,
		})
	}

	// starting -> failed -> success -> failed

	handleResult(ct.State.StartedAt.Add(1*time.Second), 1)
	expect("health_status: unhealthy")

	handleResult(ct.State.StartedAt.Add(2*time.Second), 0)
	expect("health_status: healthy")

	handleResult(ct.State.StartedAt.Add(3*time.Second), 1)
	expect("health_status: unhealthy")

	// Test retries

	reset(ct)
	ct.Config.Healthcheck.Retries = 3

	handleResult(ct.State.StartedAt.Add(20*time.Second), 1)
	handleResult(ct.State.StartedAt.Add(40*time.Second), 1)
	if ct.State.Health.Status != types.Starting {
		c.Errorf("Expecting starting, but got %#v\n", ct.State.Health.Status)
	}
	if ct.State.Health.FailingStreak != 2 {
		c.Errorf("Expecting FailingStreak=2, but got %d\n", ct.State.Health.FailingStreak)
	}
	handleResult(ct.State.StartedAt.Add(60*time.Second), 1)
	expect("health_status: unhealthy")

	handleResult(ct.State.StartedAt.Add(80*time.Second), 0)
	expect("health_status: healthy")
	if ct.State.Health.FailingStreak != 0 {
		c.Errorf("Expecting FailingStreak=0, but got %d\n", ct.State.Health.FailingStreak)
	}
}
