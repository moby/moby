// +build linux,cgo,!static_build,journald,!arm

package journald

// #cgo pkg-config: libsystemd-journal
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
//static int get_message(sd_journal *j, const char **msg, size_t *length)
//{
//	int rc;
//	*msg = NULL;
//	*length = 0;
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
//static int wait_for_data_or_close(sd_journal *j, int pipefd)
//{
//	struct pollfd fds[2];
//	uint64_t when = 0;
//	int timeout, jevents, i;
//	struct timespec ts;
//	uint64_t now;
//	do {
//		memset(&fds, 0, sizeof(fds));
//		fds[0].fd = pipefd;
//		fds[0].events = POLLHUP;
//		fds[1].fd = sd_journal_get_fd(j);
//		if (fds[1].fd < 0) {
//			return -1;
//		}
//		jevents = sd_journal_get_events(j);
//		if (jevents < 0) {
//			return -1;
//		}
//		fds[1].events = jevents;
//		sd_journal_get_timeout(j, &when);
//		if (when == -1) {
//			timeout = -1;
//		} else {
//			clock_gettime(CLOCK_MONOTONIC, &ts);
//			now = (uint64_t) ts.tv_sec * 1000000 + ts.tv_nsec / 1000;
//			timeout = when > now ? (int) ((when - now + 999) / 1000) : 0;
//		}
//		i = poll(fds, 2, timeout);
//		if ((i == -1) && (errno != EINTR)) {
//			/* An unexpected error. */
//			return -1;
//		}
//		if (fds[0].revents & POLLHUP) {
//			/* The close notification pipe was closed. */
//			return 0;
//		}
//		if (sd_journal_process(j) == SD_JOURNAL_APPEND) {
//			/* Data, which we might care about, was appended. */
//			return 1;
//		}
//	} while ((fds[0].revents & POLLHUP) == 0);
//	return 0;
//}
import "C"

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/coreos/go-systemd/journal"
	"github.com/docker/docker/daemon/logger"
)

func (s *journald) Close() error {
	s.readers.mu.Lock()
	for reader := range s.readers.readers {
		reader.Close()
	}
	s.readers.mu.Unlock()
	return nil
}

func (s *journald) drainJournal(logWatcher *logger.LogWatcher, config logger.ReadConfig, j *C.sd_journal, oldCursor string) string {
	var msg, cursor *C.char
	var length C.size_t
	var stamp C.uint64_t
	var priority C.int

	// Walk the journal from here forward until we run out of new entries.
drain:
	for {
		// Try not to send a given entry twice.
		if oldCursor != "" {
			ccursor := C.CString(oldCursor)
			defer C.free(unsafe.Pointer(ccursor))
			for C.sd_journal_test_cursor(j, ccursor) > 0 {
				if C.sd_journal_next(j) <= 0 {
					break drain
				}
			}
		}
		// Read and send the logged message, if there is one to read.
		i := C.get_message(j, &msg, &length)
		if i != -C.ENOENT && i != -C.EADDRNOTAVAIL {
			// Read the entry's timestamp.
			if C.sd_journal_get_realtime_usec(j, &stamp) != 0 {
				break
			}
			// Set up the time and text of the entry.
			timestamp := time.Unix(int64(stamp)/1000000, (int64(stamp)%1000000)*1000)
			line := append(C.GoBytes(unsafe.Pointer(msg), C.int(length)), "\n"...)
			// Recover the stream name by mapping
			// from the journal priority back to
			// the stream that we would have
			// assigned that value.
			source := ""
			if C.get_priority(j, &priority) != 0 {
				source = ""
			} else if priority == C.int(journal.PriErr) {
				source = "stderr"
			} else if priority == C.int(journal.PriInfo) {
				source = "stdout"
			}
			// Send the log message.
			cid := s.vars["CONTAINER_ID_FULL"]
			logWatcher.Msg <- &logger.Message{ContainerID: cid, Line: line, Source: source, Timestamp: timestamp}
		}
		// If we're at the end of the journal, we're done (for now).
		if C.sd_journal_next(j) <= 0 {
			break
		}
	}
	retCursor := ""
	if C.sd_journal_get_cursor(j, &cursor) == 0 {
		retCursor = C.GoString(cursor)
		C.free(unsafe.Pointer(cursor))
	}
	return retCursor
}

func (s *journald) followJournal(logWatcher *logger.LogWatcher, config logger.ReadConfig, j *C.sd_journal, pfd [2]C.int, cursor string) {
	go func() {
		// Keep copying journal data out until we're notified to stop.
		for C.wait_for_data_or_close(j, pfd[0]) == 1 {
			cursor = s.drainJournal(logWatcher, config, j, cursor)
		}
		// Clean up.
		C.close(pfd[0])
		s.readers.mu.Lock()
		delete(s.readers.readers, logWatcher)
		s.readers.mu.Unlock()
	}()
	s.readers.mu.Lock()
	s.readers.readers[logWatcher] = logWatcher
	s.readers.mu.Unlock()
	// Wait until we're told to stop.
	select {
	case <-logWatcher.WatchClose():
		// Notify the other goroutine that its work is done.
		C.close(pfd[1])
	}
}

func (s *journald) readLogs(logWatcher *logger.LogWatcher, config logger.ReadConfig) {
	var j *C.sd_journal
	var cmatch *C.char
	var stamp C.uint64_t
	var sinceUnixMicro uint64
	var pipes [2]C.int
	cursor := ""

	defer close(logWatcher.Msg)
	// Get a handle to the journal.
	rc := C.sd_journal_open(&j, C.int(0))
	if rc != 0 {
		logWatcher.Err <- fmt.Errorf("error opening journal")
		return
	}
	defer C.sd_journal_close(j)
	// Remove limits on the size of data items that we'll retrieve.
	rc = C.sd_journal_set_data_threshold(j, C.size_t(0))
	if rc != 0 {
		logWatcher.Err <- fmt.Errorf("error setting journal data threshold")
		return
	}
	// Add a match to have the library do the searching for us.
	cmatch = C.CString("CONTAINER_ID_FULL=" + s.vars["CONTAINER_ID_FULL"])
	defer C.free(unsafe.Pointer(cmatch))
	rc = C.sd_journal_add_match(j, unsafe.Pointer(cmatch), C.strlen(cmatch))
	if rc != 0 {
		logWatcher.Err <- fmt.Errorf("error setting journal match")
		return
	}
	// If we have a cutoff time, convert it to Unix time once.
	if !config.Since.IsZero() {
		nano := config.Since.UnixNano()
		sinceUnixMicro = uint64(nano / 1000)
	}
	if config.Tail > 0 {
		lines := config.Tail
		// Start at the end of the journal.
		if C.sd_journal_seek_tail(j) < 0 {
			logWatcher.Err <- fmt.Errorf("error seeking to end of journal")
			return
		}
		if C.sd_journal_previous(j) < 0 {
			logWatcher.Err <- fmt.Errorf("error backtracking to previous journal entry")
			return
		}
		// Walk backward.
		for lines > 0 {
			// Stop if the entry time is before our cutoff.
			// We'll need the entry time if it isn't, so go
			// ahead and parse it now.
			if C.sd_journal_get_realtime_usec(j, &stamp) != 0 {
				break
			} else {
				// Compare the timestamp on the entry
				// to our threshold value.
				if sinceUnixMicro != 0 && sinceUnixMicro > uint64(stamp) {
					break
				}
			}
			lines--
			// If we're at the start of the journal, or
			// don't need to back up past any more entries,
			// stop.
			if lines == 0 || C.sd_journal_previous(j) <= 0 {
				break
			}
		}
	} else {
		// Start at the beginning of the journal.
		if C.sd_journal_seek_head(j) < 0 {
			logWatcher.Err <- fmt.Errorf("error seeking to start of journal")
			return
		}
		// If we have a cutoff date, fast-forward to it.
		if sinceUnixMicro != 0 && C.sd_journal_seek_realtime_usec(j, C.uint64_t(sinceUnixMicro)) != 0 {
			logWatcher.Err <- fmt.Errorf("error seeking to start time in journal")
			return
		}
		if C.sd_journal_next(j) < 0 {
			logWatcher.Err <- fmt.Errorf("error skipping to next journal entry")
			return
		}
	}
	cursor = s.drainJournal(logWatcher, config, j, "")
	if config.Follow {
		// Create a pipe that we can poll at the same time as the journald descriptor.
		if C.pipe(&pipes[0]) == C.int(-1) {
			logWatcher.Err <- fmt.Errorf("error opening journald close notification pipe")
		} else {
			s.followJournal(logWatcher, config, j, pipes, cursor)
		}
	}
	return
}

func (s *journald) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	logWatcher := logger.NewLogWatcher()
	go s.readLogs(logWatcher, config)
	return logWatcher
}
