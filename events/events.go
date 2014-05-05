package events

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
)

type Logger struct {
	sync.RWMutex
	listeners map[int64]chan utils.JSONMessage
	events    []utils.JSONMessage
}

func NewLogger() *Logger {
	return &Logger{
		listeners: make(map[int64]chan utils.JSONMessage),
	}
}

func (l *Logger) Install(eng *engine.Engine) error {
	eng.Register("events", l.Events)
	eng.Register("logevent", l.LogEvent)
	eng.Register("events_info", l.Info)
	return nil
}

func (l *Logger) Info(job *engine.Job) engine.Status {
	info := &engine.Env{}
	info.SetInt("NEventsListener", len(l.listeners))
	if _, err := info.WriteTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (l *Logger) LogEvent(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("usage: %s ACTION ID FROM", job.Name)
	}
	jm := utils.JSONMessage{
		// FIXME: why Status and not Action?
		Status: job.Args[0],
		ID:     job.Args[1],
		From:   job.Args[2],
		Time:   time.Now().UTC().Unix(),
	}
	l.addEvent(jm)
	for _, c := range l.listeners {
		select { // non blocking channel
		case c <- jm:
		default:
		}
	}
	return engine.StatusOK
}

// FIXME: pass a pointer to avoid unnecessary copy
func (l *Logger) addEvent(jm utils.JSONMessage) {
	l.Lock()
	defer l.Unlock()
	l.events = append(l.events, jm)
}

func (l *Logger) Events(job *engine.Job) engine.Status {
	if len(job.Args) != 0 {
		return job.Errorf("Usage: %s", job.Name)
	}

	var (
		from    = time.Now().UTC().UnixNano()
		since   = job.GetenvInt64("since")
		until   = job.GetenvInt64("until")
		timeout = make(chan time.Time)
	)
	// If 'until' is set, create a timer.
	if until != 0 {
		time.AfterFunc(time.Unix(until, 0).Sub(time.Now()), func() { close(timeout) })
	} else {
		defer close(timeout)
	}
	sendEvent := func(event *utils.JSONMessage) error {
		b, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("JSON error")
		}
		_, err = job.Stdout.Write(b)
		if err != nil {
			// On error, evict the listener
			utils.Errorf("%s", err)
			l.Lock()
			delete(l.listeners, from)
			l.Unlock()
			return err
		}
		return nil
	}

	listener := make(chan utils.JSONMessage)
	l.Lock()
	if old, ok := l.listeners[from]; ok {
		delete(l.listeners, from)
		close(old)
	}
	l.listeners[from] = listener
	l.Unlock()
	job.Stdout.Write(nil) // flush
	if since != 0 {
		// If since, send previous events that happened after the timestamp and until timestamp
		for _, event := range l.getEvents() {
			if event.Time >= since && (event.Time <= until || until == 0) {
				err := sendEvent(&event)
				if err != nil && err.Error() == "JSON error" {
					continue
				}
				if err != nil {
					job.Error(err)
					return engine.StatusErr
				}
			}
		}
	}
	for {
		select {
		case event := <-listener:
			err := sendEvent(&event)
			if err != nil && err.Error() == "JSON error" {
				continue
			}
			if err != nil {
				return job.Error(err)
			}
		case <-timeout:
			return engine.StatusOK
		}
	}
	return engine.StatusOK
}

func (l *Logger) getEvents() []utils.JSONMessage {
	l.RLock()
	defer l.RUnlock()
	return l.events
}
