package events

import (
	"encoding/json"
	"github.com/dotcloud/docker/engine"
	"io"
	"testing"
	"time"
)

func TestLogEvent(t *testing.T) {
	eng := engine.New()
	if err := NewLogger().Install(eng); err != nil {
		t.Fatal(err)
	}
	events := eng.Job("events", "TestLogEvent")
	streams, err := events.Stdout.AddPipe()
	if err != nil {
		t.Fatal(err)
	}
	// Send the 1st event before we start listening.
	// Make sure that message is not received.
	eng.Job("logevent", "wrong_action", "foo", "bar").Run()
	// FIXME: there is no easy way to interrupt this job.
	go func() {
		if err := events.Run(); err != nil {
			t.Fatal(err)
		}
	}()
	waitquery := make(chan bool)
	var n int
	go func() {
		defer close(waitquery)
		d := json.NewDecoder(streams)
		for {
			e := make(map[string]interface{})
			if err := d.Decode(&e); err == io.EOF {
				return
			} else if err != nil {
				t.Fatal(err)
			}
			from, ok := e["from"]
			if !ok {
				t.Fatalf("%v", e)
			}
			// NOTE: for an unknown historical reason, "action" is stored in
			// a field called "status".
			// We test for this behavior, but encourage changing it in the future.
			action, ok := e["status"]
			if !ok {
				t.Fatalf("%v", e)
			}
			_, ok = e["id"]
			if !ok {
				t.Fatalf("%v", e)
			}
			if from != "TestLogEvent" {
				t.Fatalf("%v", e)
			}
			if action == "wrong_action" {
				t.Fatalf("%v", e)
			}
			if action != "action1" && action != "action2" && action != "action3" {
				t.Fatalf("%v", e)
			}
			n++
			if n == 3 {
				break
			}
		}
	}()
	// FIXME: currently there is no easy way for the caller to know
	// when "events" is effectively receiving messages. As a result
	// there is a race between a) the moment we call "events"
	// and b) the moment we start sending messages after that.
	//
	// As a workaround we wait 100ms. This means the test is racy
	// and may fail for no good reason, for example if the system
	// is very loaded.
	// The solution is to implement synchronization in the "events"
	// call, for example by having it send a response stream with
	// the events, and have the caller wait for that.
	time.Sleep(1000 * time.Millisecond)
	eng.Job("logevent", "action1", "foo", "TestLogEvent").Run()
	// Let's approximate a long-running command
	time.Sleep(100 * time.Millisecond)
	eng.Job("logevent", "action2", "bar", "TestLogEvent").Run()
	// Send an event with another FROM than TestLogEvent.
	// Make sure it is not received.
	eng.Job("logevent", "some other action", "naz", "another source").Run()
	eng.Job("logevent", "action3", "something with spaces", "TestLogEvent").Run()
	timeout := time.After(1 * time.Second)
	select {
	case <-timeout:
		t.Fatalf("timeout")
	case <-waitquery:
	}
	if n != 3 {
		t.Fatalf("received %d events", n)
	}
}
