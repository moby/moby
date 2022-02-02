//go:build linux && cgo && !static_build && journald
// +build linux,cgo,!static_build,journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

import (
	"errors"
	"runtime"
	"strconv"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/journald/internal/sdjournal"
)

// Fields which we know are not user-provided attribute fields.
var wellKnownFields = map[string]bool{
	"MESSAGE":             true,
	"MESSAGE_ID":          true,
	"PRIORITY":            true,
	"CODE_FILE":           true,
	"CODE_LINE":           true,
	"CODE_FUNC":           true,
	"ERRNO":               true,
	"SYSLOG_FACILITY":     true,
	fieldSyslogIdentifier: true,
	"SYSLOG_PID":          true,
	fieldSyslogTimestamp:  true,
	fieldContainerName:    true,
	fieldContainerID:      true,
	fieldContainerIDFull:  true,
	fieldContainerTag:     true,
	fieldImageName:        true,
	fieldPLogID:           true,
	fieldPLogOrdinal:      true,
	fieldPLogLast:         true,
	fieldPartialMessage:   true,
}

func getMessage(d map[string]string) (line []byte, ok bool) {
	m, ok := d["MESSAGE"]
	if ok {
		line = []byte(m)
		if d[fieldPartialMessage] != "true" {
			line = append(line, "\n"...)
		}
	}
	return line, ok
}

func getPriority(d map[string]string) (journal.Priority, bool) {
	if pri, ok := d["PRIORITY"]; ok {
		i, err := strconv.Atoi(pri)
		return journal.Priority(i), err == nil
	}
	return -1, false
}

// getSource recovers the stream name from the entry data by mapping from the
// journal priority field back to the stream that we would have assigned that
// value.
func getSource(d map[string]string) string {
	source := ""
	if priority, ok := getPriority(d); ok {
		if priority == journal.PriErr {
			source = "stderr"
		} else if priority == journal.PriInfo {
			source = "stdout"
		}
	}
	return source
}

func getAttrs(d map[string]string) []backend.LogAttr {
	var attrs []backend.LogAttr
	for k, v := range d {
		if k[0] != '_' && !wellKnownFields[k] {
			attrs = append(attrs, backend.LogAttr{Key: k, Value: v})
		}
	}
	return attrs
}

// errDrainDone is the error returned by drainJournal to signal that there are
// no more log entries to send to the log watcher.
var errDrainDone = errors.New("journald drain done")

// drainJournal reads and sends log messages from the journal. It returns the
// number of log messages sent and any error encountered. When initial != nil
// it initializes the journal read position to the position specified by config
// before reading. Otherwise it continues to read from the current position.
//
// drainJournal returns err == errDrainDone when a terminal stopping condition
// has been reached: either the watch consumer is gone or a log entry is read
// which has a timestamp after until (if until is nonzero). If the end of the
// journal is reached without encountering a terminal stopping condition,
// err == nil is returned.
func (s *journald) drainJournal(logWatcher *logger.LogWatcher, j *sdjournal.Journal, config logger.ReadConfig, initial chan struct{}) (int, error) {
	if initial != nil {
		defer func() {
			if initial != nil {
				close(initial)
			}
		}()

		var (
			err          error
			seekedToTail bool
		)
		if config.Tail >= 0 {
			if config.Until.IsZero() {
				err = j.SeekTail()
				seekedToTail = true
			} else {
				err = j.SeekRealtime(config.Until)
			}
		} else {
			if config.Since.IsZero() {
				err = j.SeekHead()
			} else {
				err = j.SeekRealtime(config.Since)
			}
		}
		if err != nil {
			return 0, err
		}

		// SeekTail() followed by Next() behaves incorrectly, so we need
		// to work around the bug by ensuring the first discrete
		// movement of the read pointer is Previous() or PreviousSkip().
		// PreviousSkip() is called inside the loop when config.Tail > 0
		// so the only special case requiring special handling is
		// config.Tail == 0.
		// https://github.com/systemd/systemd/issues/9934
		if seekedToTail && config.Tail == 0 {
			// Resolve the read pointer to the last entry in the
			// journal so that the call to Next() inside the loop
			// advances past it.
			if ok, err := j.Previous(); err != nil || !ok {
				return 0, err
			}
		}
	}

	var sent int
	for i := 0; ; i++ {
		if initial != nil && i == 0 && config.Tail > 0 {
			if n, err := j.PreviousSkip(uint(config.Tail)); err != nil || n == 0 {
				return sent, err
			}
		} else if ok, err := j.Next(); err != nil || !ok {
			return sent, err
		}

		if initial != nil && i == 0 {
			// The cursor is in position. Signal that the watcher is
			// initialized.
			close(initial)
			initial = nil // Prevent double-closing.
		}

		// Read the entry's timestamp.
		timestamp, err := j.Realtime()
		if err != nil {
			return sent, err
		}
		if timestamp.Before(config.Since) {
			if initial != nil && i == 0 && config.Tail > 0 {
				// PreviousSkip went too far back. Seek forwards.
				j.SeekRealtime(config.Since)
			}
			continue
		}
		if !config.Until.IsZero() && config.Until.Before(timestamp) {
			return sent, errDrainDone
		}

		// Read and send the logged message, if there is one to read.
		data, err := j.Data()
		if err != nil {
			return sent, err
		}
		if line, ok := getMessage(data); ok {
			// Send the log message, unless the consumer is gone
			msg := &logger.Message{
				Line:      line,
				Source:    getSource(data),
				Timestamp: timestamp.In(time.UTC),
				Attrs:     getAttrs(data),
			}
			// The daemon timestamp will differ from the "trusted"
			// timestamp of when the event was received by journald.
			// We can efficiently seek around the journal by the
			// event timestamp, and the event timestamp is what
			// journalctl displays. The daemon timestamp is just an
			// application-supplied field with no special
			// significance; libsystemd won't help us seek to the
			// entry with the closest timestamp.
			/*
				if sts := data["SYSLOG_TIMESTAMP"]; sts != "" {
					if tv, err := time.Parse(time.RFC3339Nano, sts); err == nil {
						msg.Timestamp = tv
					}
				}
			*/
			select {
			case <-logWatcher.WatchConsumerGone():
				return sent, errDrainDone
			case logWatcher.Msg <- msg:
				sent++
			}
		}

		// Call sd_journal_process() periodically during the processing loop
		// to close any opened file descriptors for rotated (deleted) journal files.
		if i != 0 && i%1024 == 0 {
			if _, err := j.Process(); err != nil {
				// log a warning but ignore it for now
				logrus.WithField("container", s.vars[fieldContainerIDFull]).
					WithField("error", err).
					Warn("journald: error processing journal")
			}
		}
	}
}

func (s *journald) readJournal(logWatcher *logger.LogWatcher, j *sdjournal.Journal, config logger.ReadConfig, ready chan struct{}) error {
	if _, err := s.drainJournal(logWatcher, j, config, ready /* initial */); err != nil {
		if err != errDrainDone {
			return err
		}
		return nil
	}
	if !config.Follow {
		return nil
	}

	for {
		status, err := j.Wait(250 * time.Millisecond)
		if err != nil {
			return err
		}
		select {
		case <-logWatcher.WatchConsumerGone():
			return nil // won't be able to write anything anymore
		case <-s.closed:
			// container is gone, drain journal
		default:
			// container is still alive
			if status == sdjournal.StatusNOP {
				// no new data -- keep waiting
				continue
			}
		}
		n, err := s.drainJournal(logWatcher, j, config, nil /* initial */)
		if err != nil {
			if err != errDrainDone {
				return err
			}
			return nil
		} else if status == sdjournal.StatusNOP && n == 0 {
			return nil
		}
	}
}

func (s *journald) readLogs(logWatcher *logger.LogWatcher, config logger.ReadConfig, ready chan struct{}) {
	defer close(logWatcher.Msg)

	// Make sure the ready channel is closed in the event of an early
	// return.
	defer func() {
		select {
		case <-ready:
		default:
			close(ready)
		}
	}()

	// Quoting https://www.freedesktop.org/software/systemd/man/sd-journal.html:
	//     Functions that operate on sd_journal objects are thread
	//     agnostic — given sd_journal pointer may only be used from one
	//     specific thread at all times (and it has to be the very same one
	//     during the entire lifetime of the object), but multiple,
	//     independent threads may use multiple, independent objects safely.
	//
	// sdjournal.Open returns a value which wraps an sd_journal pointer so
	// we need to abide by those rules.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Get a handle to the journal.
	var (
		j   *sdjournal.Journal
		err error
	)
	if s.journalReadDir != "" {
		j, err = sdjournal.OpenDir(s.journalReadDir, 0)
	} else {
		j, err = sdjournal.Open(0)
	}
	if err != nil {
		logWatcher.Err <- err
		return
	}
	defer j.Close()

	if config.Follow {
		// Initialize library inotify watches early
		if err := j.InitializeInotify(); err != nil {
			logWatcher.Err <- err
			return
		}
	}

	// Remove limits on the size of data items that we'll retrieve.
	if err := j.SetDataThreshold(0); err != nil {
		logWatcher.Err <- err
		return
	}
	// Add a match to have the library do the searching for us.
	if err := j.AddMatch(fieldContainerIDFull, s.vars[fieldContainerIDFull]); err != nil {
		logWatcher.Err <- err
		return
	}

	if err := s.readJournal(logWatcher, j, config, ready); err != nil {
		logWatcher.Err <- err
		return
	}
}

func (s *journald) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()
	ready := make(chan struct{})
	go s.readLogs(logWatcher, config, ready)
	// Block until the reader is in position to read from the current config
	// location to prevent race conditions in tests.
	<-ready
	return logWatcher
}
