package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/utils"
)

func TestEventsPublish(t *testing.T) {
	e := New()
	l1 := make(chan *utils.JSONMessage)
	l2 := make(chan *utils.JSONMessage)
	e.subscribe(l1)
	e.subscribe(l2)
	count := e.subscribersCount()
	if count != 2 {
		t.Fatalf("Must be 2 subscribers, got %d", count)
	}
	go e.log("test", "cont", "image")
	select {
	case msg := <-l1:
		if len(e.events) != 1 {
			t.Fatalf("Must be only one event, got %d", len(e.events))
		}
		if msg.Status != "test" {
			t.Fatalf("Status should be test, got %s", msg.Status)
		}
		if msg.ID != "cont" {
			t.Fatalf("ID should be cont, got %s", msg.ID)
		}
		if msg.From != "image" {
			t.Fatalf("From should be image, got %s", msg.From)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
	select {
	case msg := <-l2:
		if len(e.events) != 1 {
			t.Fatalf("Must be only one event, got %d", len(e.events))
		}
		if msg.Status != "test" {
			t.Fatalf("Status should be test, got %s", msg.Status)
		}
		if msg.ID != "cont" {
			t.Fatalf("ID should be cont, got %s", msg.ID)
		}
		if msg.From != "image" {
			t.Fatalf("From should be image, got %s", msg.From)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
}

func TestEventsPublishTimeout(t *testing.T) {
	e := New()
	l := make(chan *utils.JSONMessage)
	e.subscribe(l)

	c := make(chan struct{})
	go func() {
		e.log("test", "cont", "image")
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Timeout publishing message")
	}
}

func TestLogEvents(t *testing.T) {
	e := New()
	eng := engine.New()
	if err := e.Install(eng); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < eventsLimit+16; i++ {
		action := fmt.Sprintf("action_%d", i)
		id := fmt.Sprintf("cont_%d", i)
		from := fmt.Sprintf("image_%d", i)
		job := eng.Job("log", action, id, from)
		if err := job.Run(); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if len(e.events) != eventsLimit {
		t.Fatalf("Must be %d events, got %d", eventsLimit, len(e.events))
	}

	job := eng.Job("events")
	job.SetenvInt64("since", 1)
	job.SetenvInt64("until", time.Now().Unix())
	buf := bytes.NewBuffer(nil)
	job.Stdout.Add(buf)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	buf = bytes.NewBuffer(buf.Bytes())
	dec := json.NewDecoder(buf)
	var msgs []utils.JSONMessage
	for {
		var jm utils.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		msgs = append(msgs, jm)
	}
	if len(msgs) != eventsLimit {
		t.Fatalf("Must be %d events, got %d", eventsLimit, len(msgs))
	}
	first := msgs[0]
	if first.Status != "action_16" {
		t.Fatalf("First action is %s, must be action_15", first.Status)
	}
	last := msgs[len(msgs)-1]
	if last.Status != "action_79" {
		t.Fatalf("First action is %s, must be action_79", first.Status)
	}
}

func TestEventsCountJob(t *testing.T) {
	e := New()
	eng := engine.New()
	if err := e.Install(eng); err != nil {
		t.Fatal(err)
	}
	l1 := make(chan *utils.JSONMessage)
	l2 := make(chan *utils.JSONMessage)
	e.subscribe(l1)
	e.subscribe(l2)
	job := eng.Job("subscribers_count")
	env, _ := job.Stdout.AddEnv()
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	count := env.GetInt("count")
	if count != 2 {
		t.Fatalf("There must be 2 subscribers, got %d", count)
	}
}
