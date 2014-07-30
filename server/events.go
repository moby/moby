// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// For additional commments see server/server.go
//
package server

import (
	"encoding/json"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/utils"
)

func (srv *Server) Events(job *engine.Job) engine.Status {
	if len(job.Args) != 0 {
		return job.Errorf("Usage: %s", job.Name)
	}

	var (
		since   = job.GetenvInt64("since")
		until   = job.GetenvInt64("until")
		timeout = time.NewTimer(time.Unix(until, 0).Sub(time.Now()))
	)

	// If no until, disable timeout
	if until == 0 {
		timeout.Stop()
	}

	listener := make(chan utils.JSONMessage)
	srv.eventPublisher.Subscribe(listener)
	defer srv.eventPublisher.Unsubscribe(listener)

	// When sending an event JSON serialization errors are ignored, but all
	// other errors lead to the eviction of the listener.
	sendEvent := func(event *utils.JSONMessage) error {
		if b, err := json.Marshal(event); err == nil {
			if _, err = job.Stdout.Write(b); err != nil {
				return err
			}
		}
		return nil
	}

	job.Stdout.Write(nil)

	// Resend every event in the [since, until] time interval.
	if since != 0 {
		for _, event := range srv.GetEvents() {
			if event.Time >= since && (event.Time <= until || until == 0) {
				if err := sendEvent(&event); err != nil {
					return job.Error(err)
				}
			}
		}
	}

	for {
		select {
		case event, ok := <-listener:
			if !ok {
				return engine.StatusOK
			}
			if err := sendEvent(&event); err != nil {
				return job.Error(err)
			}
		case <-timeout.C:
			return engine.StatusOK
		}
	}
}

func (srv *Server) LogEvent(action, id, from string) *utils.JSONMessage {
	now := time.Now().UTC().Unix()
	jm := utils.JSONMessage{Status: action, ID: id, From: from, Time: now}
	srv.AddEvent(jm)
	srv.eventPublisher.Publish(jm)
	return &jm
}

func (srv *Server) AddEvent(jm utils.JSONMessage) {
	srv.Lock()
	if len(srv.events) == cap(srv.events) {
		// discard oldest event
		copy(srv.events, srv.events[1:])
		srv.events[len(srv.events)-1] = jm
	} else {
		srv.events = append(srv.events, jm)
	}
	srv.Unlock()
}

func (srv *Server) GetEvents() []utils.JSONMessage {
	srv.RLock()
	defer srv.RUnlock()
	return srv.events
}
