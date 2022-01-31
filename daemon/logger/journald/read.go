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

var wellKnownFields = map[string]bool{
	"MESSAGE":           true,
	"MESSAGE_ID":        true,
	"PRIORITY":          true,
	"CODE_FILE":         true,
	"CODE_LINE":         true,
	"CODE_FUNC":         true,
	"ERRNO":             true,
	"SYSLOG_FACILITY":   true,
	"SYSLOG_IDENTIFIER": true,
	"SYSLOG_PID":        true,
	"CONTAINER_NAME":    true,
	"CONTAINER_ID":      true,
	"CONTAINER_ID_FULL": true,
	"CONTAINER_TAG":     true,
}

func getMessage(d map[string]string) (line []byte, partial, ok bool) {
	m, ok := d["MESSAGE"]
	return []byte(m), d["CONTAINER_PARTIAL_MESSAGE"] == "true", ok
}

func getPriority(d map[string]string) (journal.Priority, bool) {
	if pri, ok := d["PRIORITY"]; ok {
		i, err := strconv.Atoi(pri)
		return journal.Priority(i), err == nil
	}
	return -1, false
}

func (s *journald) drainJournal(logWatcher *logger.LogWatcher, j *sdjournal.Journal, oldCursor *sdjournal.Cursor, until time.Time) (*sdjournal.Cursor, bool, int) {
	var (
		done  bool
		shown int
	)

	// Walk the journal from here forward until we run out of new entries
	// or we reach the until value (if provided).
drain:
	for ok := true; ok; ok, _ = j.Next() {
		// Try not to send a given entry twice.
		if oldCursor != nil {
			if ok, _ := j.TestCursor(oldCursor); ok {
				if ok, _ := j.Next(); !ok {
					break drain
				}
			}
		}
		// Read and send the logged message, if there is one to read.
		data, err := j.Data()
		if errors.Is(err, sdjournal.ErrInvalidReadPointer) {
			continue
		}
		if line, partial, ok := getMessage(data); ok {
			// Read the entry's timestamp.
			timestamp, err := j.Realtime()
			if err != nil {
				break
			}
			// Break if the timestamp exceeds any provided until flag.
			if !until.IsZero() && until.Before(timestamp) {
				done = true
				break
			}

			// Set up the text of the entry.
			if !partial {
				line = append(line, "\n"...)
			}
			// Recover the stream name by mapping
			// from the journal priority back to
			// the stream that we would have
			// assigned that value.
			source := ""
			if priority, ok := getPriority(data); ok {
				if priority == journal.PriErr {
					source = "stderr"
				} else if priority == journal.PriInfo {
					source = "stdout"
				}
			}
			// Retrieve the values of any variables we're adding to the journal.
			var attrs []backend.LogAttr
			for k, v := range data {
				if k[0] != '_' && !wellKnownFields[k] {
					attrs = append(attrs, backend.LogAttr{Key: k, Value: v})
				}
			}

			// Send the log message, unless the consumer is gone
			select {
			case <-logWatcher.WatchConsumerGone():
				done = true // we won't be able to write anything anymore
				break drain
			case logWatcher.Msg <- &logger.Message{
				Line:      line,
				Source:    source,
				Timestamp: timestamp.In(time.UTC),
				Attrs:     attrs,
			}:
				shown++
			}
			// Call sd_journal_process() periodically during the processing loop
			// to close any opened file descriptors for rotated (deleted) journal files.
			if shown%1024 == 0 {
				if _, err := j.Process(); err != nil {
					// log a warning but ignore it for now
					logrus.WithField("container", s.vars["CONTAINER_ID_FULL"]).
						WithField("error", err).
						Warn("journald: error processing journal")
				}
			}
		}
	}

	cursor, _ := j.Cursor()
	return cursor, done, shown
}

func (s *journald) followJournal(logWatcher *logger.LogWatcher, j *sdjournal.Journal, cursor *sdjournal.Cursor, until time.Time) *sdjournal.Cursor {
LOOP:
	for {
		status, err := j.Wait(250 * time.Millisecond)
		if err != nil {
			logWatcher.Err <- err
			break
		}
		select {
		case <-logWatcher.WatchConsumerGone():
			break LOOP // won't be able to write anything anymore
		case <-s.closed:
			// container is gone, drain journal
		default:
			// container is still alive
			if status == sdjournal.StatusNOP {
				// no new data -- keep waiting
				continue
			}
		}
		newCursor, done, recv := s.drainJournal(logWatcher, j, cursor, until)
		cursor.Free()
		cursor = newCursor
		if done || (status == sdjournal.StatusNOP && recv == 0) {
			break
		}
	}

	return cursor
}

func (s *journald) readLogs(logWatcher *logger.LogWatcher, config logger.ReadConfig) {
	defer close(logWatcher.Msg)

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
	j, err := sdjournal.Open(0)
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
	if err := j.AddMatch("CONTAINER_ID_FULL", s.vars["CONTAINER_ID_FULL"]); err != nil {
		logWatcher.Err <- err
		return
	}
	if config.Tail >= 0 {
		// If until time provided, start from there.
		// Otherwise start at the end of the journal.
		if !config.Until.IsZero() {
			if err := j.SeekRealtime(config.Until); err != nil {
				logWatcher.Err <- err
				return
			}
		} else if err := j.SeekTail(); err != nil {
			logWatcher.Err <- err
			return
		}
		// (Try to) skip backwards by the requested number of lines...
		if _, err := j.PreviousSkip(uint(config.Tail)); err == nil {
			// ...but not before "since"
			if !config.Since.IsZero() {
				if stamp, err := j.Realtime(); err == nil && stamp.Before(config.Since) {
					_ = j.SeekRealtime(config.Since)
				}
			}
		}
	} else {
		// Start at the beginning of the journal.
		if err := j.SeekHead(); err != nil {
			logWatcher.Err <- err
			return
		}
		// If we have a cutoff date, fast-forward to it.
		if !config.Since.IsZero() {
			if err := j.SeekRealtime(config.Since); err != nil {
				logWatcher.Err <- err
				return
			}
		}
		if _, err := j.Next(); err != nil {
			logWatcher.Err <- err
			return
		}
	}
	var cursor *sdjournal.Cursor
	if config.Tail != 0 { // special case for --tail 0
		cursor, _, _ = s.drainJournal(logWatcher, j, nil, config.Until)
	}
	if config.Follow {
		cursor = s.followJournal(logWatcher, j, cursor, config.Until)
	}
	cursor.Free()
}

func (s *journald) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()
	go s.readLogs(logWatcher, config)
	return logWatcher
}
