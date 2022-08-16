//go:build linux && cgo && !static_build && journald
// +build linux,cgo,!static_build,journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

import (
	"errors"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/journald/internal/sdjournal"
)

const closedDrainTimeout = 5 * time.Second

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
	fieldLogEpoch:         true,
	fieldLogOrdinal:       true,
}

type reader struct {
	s           *journald
	j           *sdjournal.Journal
	logWatcher  *logger.LogWatcher
	config      logger.ReadConfig
	maxOrdinal  uint64
	initialized bool
	ready       chan struct{}
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

// drainJournal reads and sends log messages from the journal.
//
// drainJournal returns errDrainDone when a terminal stopping condition has been
// reached: the watch consumer is gone, a log entry is read which has a
// timestamp after until (if until is nonzero), or the log driver is closed and
// the last message logged has been sent from the journal. If the end of the
// journal is reached without encountering a terminal stopping condition, a nil
// error is returned.
func (r *reader) drainJournal() error {
	if !r.initialized {
		defer func() {
			r.signalReady()
			r.initialized = true
		}()

		var (
			err          error
			seekedToTail bool
		)
		if r.config.Tail >= 0 {
			if r.config.Until.IsZero() {
				err = r.j.SeekTail()
				seekedToTail = true
			} else {
				err = r.j.SeekRealtime(r.config.Until)
			}
		} else {
			if r.config.Since.IsZero() {
				err = r.j.SeekHead()
			} else {
				err = r.j.SeekRealtime(r.config.Since)
			}
		}
		if err != nil {
			return err
		}

		// SeekTail() followed by Next() behaves incorrectly, so we need
		// to work around the bug by ensuring the first discrete
		// movement of the read pointer is Previous() or PreviousSkip().
		// PreviousSkip() is called inside the loop when config.Tail > 0
		// so the only special case requiring special handling is
		// config.Tail == 0.
		// https://github.com/systemd/systemd/issues/9934
		if seekedToTail && r.config.Tail == 0 {
			// Resolve the read pointer to the last entry in the
			// journal so that the call to Next() inside the loop
			// advances past it.
			if ok, err := r.j.Previous(); err != nil || !ok {
				return err
			}
		}
	}

	for i := 0; ; i++ {
		if !r.initialized && i == 0 && r.config.Tail > 0 {
			if n, err := r.j.PreviousSkip(uint(r.config.Tail)); err != nil || n == 0 {
				return err
			}
		} else if ok, err := r.j.Next(); err != nil || !ok {
			return err
		}

		if !r.initialized && i == 0 {
			// The cursor is in a position which will be unaffected
			// by subsequent logging.
			r.signalReady()
		}

		// Read the entry's timestamp.
		timestamp, err := r.j.Realtime()
		if err != nil {
			return err
		}
		// Check if the PreviousSkip went too far back. Check only the
		// initial position as we are comparing wall-clock timestamps,
		// which may not be monotonic. We don't want to skip over
		// messages sent later in time just because the clock moved
		// backwards.
		if !r.initialized && i == 0 && r.config.Tail > 0 && timestamp.Before(r.config.Since) {
			r.j.SeekRealtime(r.config.Since)
			continue
		}
		if !r.config.Until.IsZero() && r.config.Until.Before(timestamp) {
			return errDrainDone
		}

		// Read and send the logged message, if there is one to read.
		data, err := r.j.Data()
		if err != nil {
			return err
		}

		if data[fieldLogEpoch] == r.s.epoch {
			seq, err := strconv.ParseUint(data[fieldLogOrdinal], 10, 64)
			if err == nil && seq > r.maxOrdinal {
				r.maxOrdinal = seq
			}
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
			case <-r.logWatcher.WatchConsumerGone():
				return errDrainDone
			case r.logWatcher.Msg <- msg:
			}
		}

		// Call sd_journal_process() periodically during the processing loop
		// to close any opened file descriptors for rotated (deleted) journal files.
		if i != 0 && i%1024 == 0 {
			if _, err := r.j.Process(); err != nil {
				// log a warning but ignore it for now
				logrus.WithField("container", r.s.vars[fieldContainerIDFull]).
					WithField("error", err).
					Warn("journald: error processing journal")
			}
		}
	}
}

func (r *reader) readJournal() error {
	caughtUp := atomic.LoadUint64(&r.s.ordinal)
	if err := r.drainJournal(); err != nil {
		if err != errDrainDone {
			return err
		}
		return nil
	}

	var drainTimeout <-chan time.Time
	if !r.config.Follow {
		if r.s.readSyncTimeout == 0 {
			return nil
		}
		tmr := time.NewTimer(r.s.readSyncTimeout)
		defer tmr.Stop()
		drainTimeout = tmr.C
	}

	for {
		status, err := r.j.Wait(250 * time.Millisecond)
		if err != nil {
			return err
		}
		select {
		case <-r.logWatcher.WatchConsumerGone():
			return nil // won't be able to write anything anymore
		case <-drainTimeout:
			// Container is gone but we haven't found the end of the
			// logs within the timeout. Maybe it was dropped by
			// journald, e.g. due to rate-limiting.
			return nil
		case <-r.s.closed:
			// container is gone, drain journal
			lastSeq := atomic.LoadUint64(&r.s.ordinal)
			if r.maxOrdinal >= lastSeq {
				// All caught up with the logger!
				return nil
			}
			if drainTimeout == nil {
				tmr := time.NewTimer(closedDrainTimeout)
				defer tmr.Stop()
				drainTimeout = tmr.C
			}
		default:
			// container is still alive
			if status == sdjournal.StatusNOP {
				// no new data -- keep waiting
				continue
			}
		}
		err = r.drainJournal()
		if err != nil {
			if err != errDrainDone {
				return err
			}
			return nil
		}
		if !r.config.Follow && r.s.readSyncTimeout > 0 && r.maxOrdinal >= caughtUp {
			return nil
		}
	}
}

func (r *reader) readLogs() {
	defer close(r.logWatcher.Msg)

	// Make sure the ready channel is closed in the event of an early
	// return.
	defer r.signalReady()

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
	var err error
	if r.s.journalReadDir != "" {
		r.j, err = sdjournal.OpenDir(r.s.journalReadDir, 0)
	} else {
		r.j, err = sdjournal.Open(0)
	}
	if err != nil {
		r.logWatcher.Err <- err
		return
	}
	defer r.j.Close()

	if r.config.Follow {
		// Initialize library inotify watches early
		if err := r.j.InitializeInotify(); err != nil {
			r.logWatcher.Err <- err
			return
		}
	}

	// Remove limits on the size of data items that we'll retrieve.
	if err := r.j.SetDataThreshold(0); err != nil {
		r.logWatcher.Err <- err
		return
	}
	// Add a match to have the library do the searching for us.
	if err := r.j.AddMatch(fieldContainerIDFull, r.s.vars[fieldContainerIDFull]); err != nil {
		r.logWatcher.Err <- err
		return
	}

	if err := r.readJournal(); err != nil {
		r.logWatcher.Err <- err
		return
	}
}

func (r *reader) signalReady() {
	select {
	case <-r.ready:
	default:
		close(r.ready)
	}
}

func (s *journald) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	r := &reader{
		s:          s,
		logWatcher: logger.NewLogWatcher(),
		config:     config,
		ready:      make(chan struct{}),
	}
	go r.readLogs()
	// Block until the reader is in position to read from the current config
	// location to prevent race conditions in tests.
	<-r.ready
	return r.logWatcher
}

func waitUntilFlushedImpl(s *journald) error {
	if s.readSyncTimeout == 0 {
		return nil
	}

	ordinal := atomic.LoadUint64(&s.ordinal)
	if ordinal == 0 {
		return nil // No logs were sent; nothing to wait for.
	}

	flushed := make(chan error)
	go func() {
		defer close(flushed)
		runtime.LockOSThread()

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
			flushed <- err
			return
		}
		defer j.Close()

		if err := j.AddMatch(fieldContainerIDFull, s.vars[fieldContainerIDFull]); err != nil {
			flushed <- err
			return
		}
		if err := j.AddMatch(fieldLogEpoch, s.epoch); err != nil {
			flushed <- err
			return
		}
		if err := j.AddMatch(fieldLogOrdinal, strconv.FormatUint(ordinal, 10)); err != nil {
			flushed <- err
			return
		}

		deadline := time.Now().Add(s.readSyncTimeout)
		for time.Now().Before(deadline) {
			if ok, err := j.Next(); ok {
				// Found it!
				return
			} else if err != nil {
				flushed <- err
				return
			}
			if _, err := j.Wait(100 * time.Millisecond); err != nil {
				flushed <- err
				return
			}
		}
		logrus.WithField("container", s.vars[fieldContainerIDFull]).
			Warn("journald: deadline exceeded waiting for logs to be committed to journal")
	}()
	return <-flushed
}

func init() {
	waitUntilFlushed = waitUntilFlushedImpl
}
