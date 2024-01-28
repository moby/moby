//go:build linux && cgo && !static_build && journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

import (
	"context"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/containerd/log"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/journald/internal/sdjournal"
)

const (
	closedDrainTimeout = 5 * time.Second
	waitInterval       = 250 * time.Millisecond
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
	fieldLogEpoch:         true,
	fieldLogOrdinal:       true,
}

type reader struct {
	s             *journald
	j             *sdjournal.Journal
	logWatcher    *logger.LogWatcher
	config        logger.ReadConfig
	maxOrdinal    uint64
	ready         chan struct{}
	drainDeadline time.Time
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

// The SeekXYZ() methods all move the journal read pointer to a "conceptual"
// position which does not correspond to any journal entry. A subsequent call to
// Next(), Previous() or similar is necessary to resolve the read pointer to a
// discrete entry.
// https://github.com/systemd/systemd/pull/5930#issuecomment-300878104
// But that's not all! If there is no discrete entry to resolve the position to,
// the call to Next() or Previous() will just leave the read pointer in a
// conceptual position, or do something even more bizarre.
// https://github.com/systemd/systemd/issues/9934

// initialSeekHead positions the journal read pointer at the earliest journal
// entry with a timestamp of at least r.config.Since. It returns true if there
// is an entry to read at the read pointer.
func (r *reader) initialSeekHead() (bool, error) {
	var err error
	if r.config.Since.IsZero() {
		err = r.j.SeekHead()
	} else {
		err = r.j.SeekRealtime(r.config.Since)
	}
	if err != nil {
		return false, err
	}
	return r.j.Next()
}

// initialSeekTail positions the journal read pointer at a journal entry
// relative to the tail of the journal at the time of the call based on the
// specification in r.config. It returns true if there is an entry to read at
// the read pointer. Otherwise the read pointer is set to a conceptual position
// which will be resolved to the desired entry (once written) by advancing
// forward with r.j.Next() or similar.
func (r *reader) initialSeekTail() (bool, error) {
	var err error
	if r.config.Until.IsZero() {
		err = r.j.SeekTail()
	} else {
		err = r.j.SeekRealtime(r.config.Until)
	}
	if err != nil {
		return false, err
	}

	var ok bool
	if r.config.Tail == 0 {
		ok, err = r.j.Previous()
	} else {
		var n int
		n, err = r.j.PreviousSkip(uint(r.config.Tail))
		ok = n > 0
	}
	if err != nil {
		return ok, err
	}
	if !ok {
		// The (filtered) journal has no entries. The tail is the head: all new
		// entries which get written into the journal from this point forward
		// should be read from the journal. However the read pointer is
		// positioned at a conceptual position which is not condusive to reading
		// those entries. The tail of the journal is resolved to the last entry
		// in the journal _at the time of the first successful Previous() call_,
		// which means that an arbitrary number of journal entries added in the
		// interim may be skipped: race condition. While the realtime conceptual
		// position is not so racy, it is also unhelpful: it is the timestamp
		// past where reading should stop, so all logs that should be followed
		// would be skipped over.
		// Reset the read pointer position to avoid these problems.
		return r.initialSeekHead()
	} else if r.config.Tail == 0 {
		// The journal read pointer is positioned at the discrete position of
		// the journal entry _before_ the entry to send.
		return r.j.Next()
	}

	// Check if the PreviousSkip went too far back.
	timestamp, err := r.j.Realtime()
	if err != nil {
		return false, err
	}
	if timestamp.Before(r.config.Since) {
		if err := r.j.SeekRealtime(r.config.Since); err != nil {
			return false, err
		}
		return r.j.Next()
	}
	return true, nil
}

// wait blocks until the journal has new data to read, the reader's drain
// deadline is exceeded, or the log reading consumer is gone.
func (r *reader) wait() (bool, error) {
	for {
		dur := waitInterval
		if !r.drainDeadline.IsZero() {
			dur = time.Until(r.drainDeadline)
			if dur < 0 {
				// Container is gone but we haven't found the end of the
				// logs before the deadline. Maybe it was dropped by
				// journald, e.g. due to rate-limiting.
				return false, nil
			} else if dur > waitInterval {
				dur = waitInterval
			}
		}
		status, err := r.j.Wait(dur)
		if err != nil {
			return false, err
		} else if status != sdjournal.StatusNOP {
			return true, nil
		}
		select {
		case <-r.logWatcher.WatchConsumerGone():
			return false, nil
		case <-r.s.closed:
			// Container is gone; don't wait indefinitely for journal entries that will never arrive.
			if r.drainDeadline.IsZero() {
				r.drainDeadline = time.Now().Add(closedDrainTimeout)
			}
		default:
		}
	}
}

// nextWait blocks until there is a new journal entry to read, and advances the
// journal read pointer to it.
func (r *reader) nextWait() (bool, error) {
	for {
		if ok, err := r.j.Next(); err != nil || ok {
			return ok, err
		}
		if ok, err := r.wait(); err != nil || !ok {
			return false, err
		}
	}
}

// drainJournal reads and sends log messages from the journal, starting from the
// current read pointer, until the end of the journal or a terminal stopping
// condition is reached.
//
// It returns false when a terminal stopping condition has been reached: the
// watch consumer is gone, a log entry is read which has a timestamp after until
// (if until is nonzero), or the log driver is closed and the last message
// logged has been sent from the journal.
func (r *reader) drainJournal() (bool, error) {
	for i := 0; ; i++ {
		// Read the entry's timestamp.
		timestamp, err := r.j.Realtime()
		if err != nil {
			return true, err
		}
		if !r.config.Until.IsZero() && r.config.Until.Before(timestamp) {
			return false, nil
		}

		// Read and send the logged message, if there is one to read.
		data, err := r.j.Data()
		if err != nil {
			return true, err
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
				return false, nil
			case r.logWatcher.Msg <- msg:
			}
		}

		// Call sd_journal_process() periodically during the processing loop
		// to close any opened file descriptors for rotated (deleted) journal files.
		if i != 0 && i%1024 == 0 {
			if _, err := r.j.Process(); err != nil {
				// log a warning but ignore it for now
				log.G(context.TODO()).WithField("container", r.s.vars[fieldContainerIDFull]).
					WithField("error", err).
					Warn("journald: error processing journal")
			}
		}

		if ok, err := r.j.Next(); err != nil || !ok {
			return true, err
		}
	}
}

func (r *reader) readJournal() error {
	caughtUp := atomic.LoadUint64(&r.s.ordinal)
	if more, err := r.drainJournal(); err != nil || !more {
		return err
	}

	if !r.config.Follow {
		if r.s.readSyncTimeout == 0 {
			return nil
		}
		r.drainDeadline = time.Now().Add(r.s.readSyncTimeout)
	}

	for {
		select {
		case <-r.s.closed:
			// container is gone, drain journal
			lastSeq := atomic.LoadUint64(&r.s.ordinal)
			if r.maxOrdinal >= lastSeq {
				// All caught up with the logger!
				return nil
			}
		default:
		}

		if more, err := r.nextWait(); err != nil || !more {
			return err
		}
		if more, err := r.drainJournal(); err != nil || !more {
			return err
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

	var ok bool
	if r.config.Tail >= 0 {
		ok, err = r.initialSeekTail()
	} else {
		ok, err = r.initialSeekHead()
	}
	if err != nil {
		r.logWatcher.Err <- err
		return
	}
	r.signalReady()
	if !ok {
		if !r.config.Follow {
			return
		}
		// Either the read pointer is positioned at a discrete journal entry, in
		// which case the position will be unaffected by subsequent logging, or
		// the read pointer is in the conceptual position corresponding to the
		// first journal entry to send once it is logged in the future.
		if more, err := r.nextWait(); err != nil || !more {
			if err != nil {
				r.logWatcher.Err <- err
			}
			return
		}
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
		defer runtime.UnlockOSThread()

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
		log.G(context.TODO()).WithField("container", s.vars[fieldContainerIDFull]).
			Warn("journald: deadline exceeded waiting for logs to be committed to journal")
	}()
	return <-flushed
}

func init() {
	waitUntilFlushed = waitUntilFlushedImpl
}
