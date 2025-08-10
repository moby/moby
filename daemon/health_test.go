package daemon

import (
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	eventtypes "github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/events"
)

func reset(c *container.Container) {
	c.State = &container.State{}
	c.Health = &container.Health{}
	c.Health.SetStatus(containertypes.Starting)
}

func TestNoneHealthcheck(t *testing.T) {
	c := &container.Container{
		ID:   "container_id",
		Name: "container_name",
		Config: &containertypes.Config{
			Image: "image_name",
			Healthcheck: &containertypes.HealthConfig{
				Test: []string{"NONE"},
			},
		},
		State: &container.State{},
	}
	store, err := container.NewViewDB()
	if err != nil {
		t.Fatal(err)
	}
	daemon := &Daemon{
		containersReplica: store,
	}

	daemon.initHealthMonitor(c)
	if c.Health != nil {
		t.Error("Expecting Health to be nil, but was not")
	}
}

// FIXME(vdemeester) This takes around 3s… This is *way* too long
func TestHealthStates(t *testing.T) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	expect := func(expected eventtypes.Action) {
		select {
		case event := <-l:
			ev := event.(eventtypes.Message)
			if ev.Action != expected {
				t.Errorf("Expecting event %#v, but got %#v\n", expected, ev.Action)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Expecting event %#v, but got nothing\n", expected)
		}
	}

	c := &container.Container{
		ID:   "container_id",
		Name: "container_name",
		Config: &containertypes.Config{
			Image: "image_name",
		},
	}

	store, err := container.NewViewDB()
	if err != nil {
		t.Fatal(err)
	}

	daemon := &Daemon{
		EventsService:     e,
		containersReplica: store,
	}
	muteLogs(t)

	c.Config.Healthcheck = &containertypes.HealthConfig{
		Retries: 1,
	}

	reset(c)

	handleResult := func(startTime time.Time, exitCode int) {
		handleProbeResult(daemon, c, &containertypes.HealthcheckResult{
			Start:    startTime,
			End:      startTime,
			ExitCode: exitCode,
		}, nil)
	}

	// starting -> failed -> success -> failed

	handleResult(c.StartedAt.Add(1*time.Second), 1)
	expect(eventtypes.ActionHealthStatusUnhealthy)

	handleResult(c.StartedAt.Add(2*time.Second), 0)
	expect(eventtypes.ActionHealthStatusHealthy)

	handleResult(c.StartedAt.Add(3*time.Second), 1)
	expect(eventtypes.ActionHealthStatusUnhealthy)

	// Test retries

	reset(c)
	c.Config.Healthcheck.Retries = 3

	handleResult(c.StartedAt.Add(20*time.Second), 1)
	handleResult(c.StartedAt.Add(40*time.Second), 1)
	if status := c.Health.Status(); status != containertypes.Starting {
		t.Errorf("Expecting starting, but got %#v\n", status)
	}
	if c.Health.FailingStreak != 2 {
		t.Errorf("Expecting FailingStreak=2, but got %d\n", c.Health.FailingStreak)
	}
	handleResult(c.StartedAt.Add(60*time.Second), 1)
	expect(eventtypes.ActionHealthStatusUnhealthy)

	handleResult(c.StartedAt.Add(80*time.Second), 0)
	expect(eventtypes.ActionHealthStatusHealthy)
	if c.Health.FailingStreak != 0 {
		t.Errorf("Expecting FailingStreak=0, but got %d\n", c.Health.FailingStreak)
	}

	// Test start period

	reset(c)
	c.Config.Healthcheck.Retries = 2
	c.Config.Healthcheck.StartPeriod = 30 * time.Second

	handleResult(c.StartedAt.Add(20*time.Second), 1)
	if status := c.Health.Status(); status != containertypes.Starting {
		t.Errorf("Expecting starting, but got %#v\n", status)
	}
	if c.Health.FailingStreak != 0 {
		t.Errorf("Expecting FailingStreak=0, but got %d\n", c.Health.FailingStreak)
	}
	handleResult(c.StartedAt.Add(50*time.Second), 1)
	if status := c.Health.Status(); status != containertypes.Starting {
		t.Errorf("Expecting starting, but got %#v\n", status)
	}
	if c.Health.FailingStreak != 1 {
		t.Errorf("Expecting FailingStreak=1, but got %d\n", c.Health.FailingStreak)
	}
	handleResult(c.StartedAt.Add(80*time.Second), 0)
	expect(eventtypes.ActionHealthStatusHealthy)
	if c.Health.FailingStreak != 0 {
		t.Errorf("Expecting FailingStreak=0, but got %d\n", c.Health.FailingStreak)
	}
}
