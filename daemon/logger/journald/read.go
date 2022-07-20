//go:build linux && cgo && !static_build && journald
// +build linux,cgo,!static_build,journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

// #include <sys/types.h>
// #include <sys/poll.h>
// #include <systemd/sd-journal.h>
// #include <errno.h>
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// #include <time.h>
// #include <unistd.h>
//
//static int get_message(sd_journal *j, const char **msg, size_t *length, int *partial)
//{
//	int rc;
//	size_t plength;
//	*msg = NULL;
//	*length = 0;
//	plength = strlen("CONTAINER_PARTIAL_MESSAGE=true");
//	rc = sd_journal_get_data(j, "CONTAINER_PARTIAL_MESSAGE", (const void **) msg, length);
//	*partial = ((rc == 0) && (*length == plength) && (memcmp(*msg, "CONTAINER_PARTIAL_MESSAGE=true", plength) == 0));
//	rc = sd_journal_get_data(j, "MESSAGE", (const void **) msg, length);
//	if (rc == 0) {
//		if (*length > 8) {
//			(*msg) += 8;
//			*length -= 8;
//		} else {
//			*msg = NULL;
//			*length = 0;
//			rc = -ENOENT;
//		}
//	}
//	return rc;
//}
//static int get_priority(sd_journal *j, int *priority)
//{
//	const void *data;
//	size_t i, length;
//	int rc;
//	*priority = -1;
//	rc = sd_journal_get_data(j, "PRIORITY", &data, &length);
//	if (rc == 0) {
//		if ((length > 9) && (strncmp(data, "PRIORITY=", 9) == 0)) {
//			*priority = 0;
//			for (i = 9; i < length; i++) {
//				*priority = *priority * 10 + ((const char *)data)[i] - '0';
//			}
//			if (length > 9) {
//				rc = 0;
//			}
//		}
//	}
//	return rc;
//}
//static int is_attribute_field(const char *msg, size_t length)
//{
//	static const struct known_field {
//		const char *name;
//		size_t length;
//	} fields[] = {
//		{"MESSAGE", sizeof("MESSAGE") - 1},
//		{"MESSAGE_ID", sizeof("MESSAGE_ID") - 1},
//		{"PRIORITY", sizeof("PRIORITY") - 1},
//		{"CODE_FILE", sizeof("CODE_FILE") - 1},
//		{"CODE_LINE", sizeof("CODE_LINE") - 1},
//		{"CODE_FUNC", sizeof("CODE_FUNC") - 1},
//		{"ERRNO", sizeof("ERRNO") - 1},
//		{"SYSLOG_FACILITY", sizeof("SYSLOG_FACILITY") - 1},
//		{"SYSLOG_IDENTIFIER", sizeof("SYSLOG_IDENTIFIER") - 1},
//		{"SYSLOG_PID", sizeof("SYSLOG_PID") - 1},
//		{"CONTAINER_NAME", sizeof("CONTAINER_NAME") - 1},
//		{"CONTAINER_ID", sizeof("CONTAINER_ID") - 1},
//		{"CONTAINER_ID_FULL", sizeof("CONTAINER_ID_FULL") - 1},
//		{"CONTAINER_TAG", sizeof("CONTAINER_TAG") - 1},
//	};
//	unsigned int i;
//	void *p;
//	if ((length < 1) || (msg[0] == '_') || ((p = memchr(msg, '=', length)) == NULL)) {
//		return -1;
//	}
//	length = ((const char *) p) - msg;
//	for (i = 0; i < sizeof(fields) / sizeof(fields[0]); i++) {
//		if ((fields[i].length == length) && (memcmp(fields[i].name, msg, length) == 0)) {
//			return -1;
//		}
//	}
//	return 0;
//}
//static int get_attribute_field(sd_journal *j, const char **msg, size_t *length)
//{
//	int rc;
//	*msg = NULL;
//	*length = 0;
//	while ((rc = sd_journal_enumerate_data(j, (const void **) msg, length)) > 0) {
//		if (is_attribute_field(*msg, *length) == 0) {
//			break;
//		}
//		rc = -ENOENT;
//	}
//	return rc;
//}
import "C"

import (
	"errors"
	"strings"
	"time"
	"unsafe"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/logger"
	"github.com/sirupsen/logrus"
)

// CErr converts error code returned from a sd_journal_* function
// (which returns -errno) to a string
func CErr(ret C.int) string {
	return C.GoString(C.strerror(-ret))
}

func (s *journald) drainJournal(logWatcher *logger.LogWatcher, j *C.sd_journal, oldCursor *C.char, untilUnixMicro uint64) (*C.char, bool, int) {
	var (
		msg, data, cursor *C.char
		length            C.size_t
		stamp             C.uint64_t
		priority, partial C.int
		done              bool
		shown             int
	)

	// Walk the journal from here forward until we run out of new entries
	// or we reach the until value (if provided).
drain:
	for {
		// Try not to send a given entry twice.
		if oldCursor != nil {
			for C.sd_journal_test_cursor(j, oldCursor) > 0 {
				if C.sd_journal_next(j) <= 0 {
					break drain
				}
			}
		}
		// Read and send the logged message, if there is one to read.
		i := C.get_message(j, &msg, &length, &partial)
		if i != -C.ENOENT && i != -C.EADDRNOTAVAIL {
			// Read the entry's timestamp.
			if C.sd_journal_get_realtime_usec(j, &stamp) != 0 {
				break
			}
			// Break if the timestamp exceeds any provided until flag.
			if untilUnixMicro != 0 && untilUnixMicro < uint64(stamp) {
				done = true
				break
			}

			// Set up the time and text of the entry.
			timestamp := time.Unix(int64(stamp)/1000000, (int64(stamp)%1000000)*1000)
			line := C.GoBytes(unsafe.Pointer(msg), C.int(length))
			if partial == 0 {
				line = append(line, "\n"...)
			}
			// Recover the stream name by mapping
			// from the journal priority back to
			// the stream that we would have
			// assigned that value.
			source := ""
			if C.get_priority(j, &priority) == 0 {
				if priority == C.int(journal.PriErr) {
					source = "stderr"
				} else if priority == C.int(journal.PriInfo) {
					source = "stdout"
				}
			}
			// Retrieve the values of any variables we're adding to the journal.
			var attrs []backend.LogAttr
			C.sd_journal_restart_data(j)
			for C.get_attribute_field(j, &data, &length) > C.int(0) {
				kv := strings.SplitN(C.GoStringN(data, C.int(length)), "=", 2)
				attrs = append(attrs, backend.LogAttr{Key: kv[0], Value: kv[1]})
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
				if ret := C.sd_journal_process(j); ret < 0 {
					// log a warning but ignore it for now
					logrus.WithField("container", s.vars["CONTAINER_ID_FULL"]).
						WithField("error", CErr(ret)).
						Warn("journald: error processing journal")
				}
			}
		}
		// If we're at the end of the journal, we're done (for now).
		if C.sd_journal_next(j) <= 0 {
			break
		}
	}

	// free(NULL) is safe
	C.free(unsafe.Pointer(oldCursor))
	if C.sd_journal_get_cursor(j, &cursor) != 0 {
		// ensure that we won't be freeing an address that's invalid
		cursor = nil
	}
	return cursor, done, shown
}

func (s *journald) followJournal(logWatcher *logger.LogWatcher, j *C.sd_journal, cursor *C.char, untilUnixMicro uint64) *C.char {
	defer close(logWatcher.Msg)

	waitTimeout := C.uint64_t(250000) // 0.25s

LOOP:
	for {
		status := C.sd_journal_wait(j, waitTimeout)
		if status < 0 {
			logWatcher.Err <- errors.New("error waiting for journal: " + CErr(status))
			break
		}
		select {
		case <-logWatcher.WatchConsumerGone():
			break LOOP // won't be able to write anything anymore
		case <-s.closed:
			// container is gone, drain journal
		default:
			// container is still alive
			if status == C.SD_JOURNAL_NOP {
				// no new data -- keep waiting
				continue
			}
		}
		newCursor, done, recv := s.drainJournal(logWatcher, j, cursor, untilUnixMicro)
		cursor = newCursor
		if done || (status == C.SD_JOURNAL_NOP && recv == 0) {
			break
		}
	}

	return cursor
}

func (s *journald) readLogs(logWatcher *logger.LogWatcher, config logger.ReadConfig) {
	var (
		j              *C.sd_journal
		cmatch, cursor *C.char
		stamp          C.uint64_t
		sinceUnixMicro uint64
		untilUnixMicro uint64
	)

	// Get a handle to the journal.
	if rc := C.sd_journal_open(&j, C.int(0)); rc != 0 {
		logWatcher.Err <- errors.New("error opening journal: " + CErr(rc))
		close(logWatcher.Msg)
		return
	}
	if config.Follow {
		// Initialize library inotify watches early
		if rc := C.sd_journal_get_fd(j); rc < 0 {
			logWatcher.Err <- errors.New("error getting journald fd: " + CErr(rc))
			close(logWatcher.Msg)
			return
		}
	}
	// If we end up following the log, we can set the journal context
	// pointer and the channel pointer to nil so that we won't close them
	// here, potentially while the goroutine that uses them is still
	// running.  Otherwise, close them when we return from this function.
	following := false
	defer func() {
		if !following {
			close(logWatcher.Msg)
		}
		C.sd_journal_close(j)
	}()
	// Remove limits on the size of data items that we'll retrieve.
	if rc := C.sd_journal_set_data_threshold(j, C.size_t(0)); rc != 0 {
		logWatcher.Err <- errors.New("error setting journal data threshold: " + CErr(rc))
		return
	}
	// Add a match to have the library do the searching for us.
	cmatch = C.CString("CONTAINER_ID_FULL=" + s.vars["CONTAINER_ID_FULL"])
	defer C.free(unsafe.Pointer(cmatch))
	if rc := C.sd_journal_add_match(j, unsafe.Pointer(cmatch), C.strlen(cmatch)); rc != 0 {
		logWatcher.Err <- errors.New("error setting journal match: " + CErr(rc))
		return
	}
	// If we have a cutoff time, convert it to Unix time once.
	if !config.Since.IsZero() {
		nano := config.Since.UnixNano()
		sinceUnixMicro = uint64(nano / 1000)
	}
	// If we have an until value, convert it too
	if !config.Until.IsZero() {
		nano := config.Until.UnixNano()
		untilUnixMicro = uint64(nano / 1000)
	}
	if config.Tail >= 0 {
		// If until time provided, start from there.
		// Otherwise start at the end of the journal.
		if untilUnixMicro != 0 {
			if rc := C.sd_journal_seek_realtime_usec(j, C.uint64_t(untilUnixMicro)); rc != 0 {
				logWatcher.Err <- errors.New("error seeking provided until value: " + CErr(rc))
				return
			}
		} else if rc := C.sd_journal_seek_tail(j); rc != 0 {
			logWatcher.Err <- errors.New("error seeking to end of journal: " + CErr(rc))
			return
		}
		// (Try to) skip backwards by the requested number of lines...
		if C.sd_journal_previous_skip(j, C.uint64_t(config.Tail)) >= 0 {
			// ...but not before "since"
			if sinceUnixMicro != 0 &&
				C.sd_journal_get_realtime_usec(j, &stamp) == 0 &&
				uint64(stamp) < sinceUnixMicro {
				C.sd_journal_seek_realtime_usec(j, C.uint64_t(sinceUnixMicro))
			}
		}
	} else {
		// Start at the beginning of the journal.
		if rc := C.sd_journal_seek_head(j); rc != 0 {
			logWatcher.Err <- errors.New("error seeking to start of journal: " + CErr(rc))
			return
		}
		// If we have a cutoff date, fast-forward to it.
		if sinceUnixMicro != 0 {
			if rc := C.sd_journal_seek_realtime_usec(j, C.uint64_t(sinceUnixMicro)); rc != 0 {
				logWatcher.Err <- errors.New("error seeking to start time in journal: " + CErr(rc))
				return
			}
		}
		if rc := C.sd_journal_next(j); rc < 0 {
			logWatcher.Err <- errors.New("error skipping to next journal entry: " + CErr(rc))
			return
		}
	}
	if config.Tail != 0 { // special case for --tail 0
		cursor, _, _ = s.drainJournal(logWatcher, j, nil, untilUnixMicro)
	}
	if config.Follow {
		cursor = s.followJournal(logWatcher, j, cursor, untilUnixMicro)
		// Let followJournal handle freeing the journal context
		// object and closing the channel.
		following = true
	}

	C.free(unsafe.Pointer(cursor))
}

func (s *journald) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()
	go s.readLogs(logWatcher, config)
	return logWatcher
}
