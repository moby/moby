package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/events"
	"github.com/docker/docker/daemon/testutils"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

func TestLogContainerEventCopyLabels(t *testing.T) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	container := &container.Container{
		ID:   "container_id",
		Name: "container_name",
		Config: &containertypes.Config{
			Image: "image_name",
			Labels: map[string]string{
				"node": "1",
				"os":   "alpine",
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

	validateTestAttributes(t, l, map[string]string{
		"node": "1",
		"os":   "alpine",
	})
}

func TestLogContainerEventWithAttributes(t *testing.T) {
	e := events.New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	container := &container.Container{
		ID:   "container_id",
		Name: "container_name",
		Config: &containertypes.Config{
			Labels: map[string]string{
				"node": "1",
				"os":   "alpine",
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

	validateTestAttributes(t, l, map[string]string{
		"node": "1",
		"foo":  "bar",
	})
}

func validateTestAttributes(t *testing.T, l chan interface{}, expectedAttributesToTest map[string]string) {
	select {
	case ev := <-l:
		event, ok := ev.(eventtypes.Message)
		if !ok {
			t.Fatalf("Unexpected event message: %q", ev)
		}
		for key, expected := range expectedAttributesToTest {
			actual, ok := event.Actor.Attributes[key]
			if !ok || actual != expected {
				t.Fatalf("Expected value for key %s to be %s, but was %s (event:%v)", key, expected, actual, event)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("LogEvent test timed out")
	}
}

func TestGenerateEmptyClusterEventShouldLogError(t *testing.T) {
	daemon := &Daemon{}

	testutils.EnableLogHook()

	events := []*swarmapi.WatchMessage_Event{}
	events = append(events, &swarmapi.WatchMessage_Event{})

	msg := &swarmapi.WatchMessage{
		Events: events,
	}

	daemon.generateClusterEvent(msg)

	entries := testutils.GetLogHookEntries()
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, logrus.ErrorLevel, entries[0].Level)
	assert.Equal(t, true, strings.HasPrefix(entries[0].Message, "event without object: "))

	testutils.DisableLogHook()
}

func TestGenerateClusterEventAndPubSubForAllClusterEventTypes(t *testing.T) {
	daemon := &Daemon{
		EventsService: events.New(),
	}

	subscriber := NewEventSubscriber(daemon)
	subscriber.subscribe()

	events := []*swarmapi.WatchMessage_Event{}
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateTask{Task: &swarmapi.Task{ID: "my-task-id"}}))
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateNode{Node: &swarmapi.Node{ID: "my-node-id"}}))
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateConfig{Config: &swarmapi.Config{ID: "my-config-id"}}))
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateSecret{Secret: &swarmapi.Secret{ID: "my-secret-id"}}))
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateNetwork{Network: &swarmapi.Network{ID: "my-network-id"}}))
	events = append(events, swarmapi.WatchMessageEvent(swarmapi.EventCreateService{Service: &swarmapi.Service{ID: "my-service-id"}}))

	msg := &swarmapi.WatchMessage{
		Events: events,
	}

	daemon.generateClusterEvent(msg)

	subscriber.stopWhenCapturedEventsReaches(6)

	captures := subscriber.getCapturedEvents()

	for _, cap := range captures {
		switch cap.Type {
		case "task":
			assert.Equal(t, cap.Actor.ID, "my-task-id")
		case "node":
			assert.Equal(t, cap.Actor.ID, "my-node-id")
		case "config":
			assert.Equal(t, cap.Actor.ID, "my-config-id")
		case "secret":
			assert.Equal(t, cap.Actor.ID, "my-secret-id")
		case "network":
			assert.Equal(t, cap.Actor.ID, "my-network-id")
		case "service":
			assert.Equal(t, cap.Actor.ID, "my-service-id")
		}
	}
}

type EventSubscriber struct {
	daemon       *Daemon
	messages     []eventtypes.Message
	eventChannel chan interface{}
	cancelFunc   func()
	ctx          context.Context
	captured     []eventtypes.Message
	numCaptures  int
}

func NewEventSubscriber(daemon *Daemon) *EventSubscriber {
	return &EventSubscriber{
		daemon:   daemon,
		captured: []eventtypes.Message{},
	}
}

func (s *EventSubscriber) subscribe() {
	s.messages, s.eventChannel = s.daemon.SubscribeToEvents(time.Now(), time.Now().AddDate(0, 0, 1), filters.NewArgs())
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())
	go s.listenToChannel()
}

func (s *EventSubscriber) listenToChannel() {
	for {
		select {
		case <-s.ctx.Done():
			s.daemon.UnsubscribeFromEvents(s.eventChannel)
			return
		case e := <-s.eventChannel:
			s.numCaptures++
			if evt, ok := e.(eventtypes.Message); ok {
				s.captured = append(s.captured, evt)
			}
		}
	}
}

func (s *EventSubscriber) getCapturedEvents() []eventtypes.Message {
	return s.captured
}

// blocking
func (s *EventSubscriber) stopWhenCapturedEventsReaches(num int) {
	for {
		if s.numCaptures == num {
			break
		}
	}

	s.stop()
}

func (s *EventSubscriber) stop() {
	s.cancelFunc()
}
