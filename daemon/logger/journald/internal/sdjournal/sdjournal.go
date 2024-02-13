//go:build linux && cgo && !static_build && journald

package sdjournal // import "github.com/docker/docker/daemon/logger/journald/internal/sdjournal"

// #cgo pkg-config: libsystemd
// #include <stdlib.h>
// #include <systemd/sd-journal.h>
//
// static int add_match(sd_journal *j, _GoString_ s) {
// 	return sd_journal_add_match(j, _GoStringPtr(s), _GoStringLen(s));
// }
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Status is an sd-journal status code.
type Status int

// Status values for Process() and Wait().
const (
	StatusNOP        = Status(C.SD_JOURNAL_NOP)        // SD_JOURNAL_NOP
	StatusAPPEND     = Status(C.SD_JOURNAL_APPEND)     // SD_JOURNAL_APPEND
	StatusINVALIDATE = Status(C.SD_JOURNAL_INVALIDATE) // SD_JOURNAL_INVALIDATE
)

const (
	// ErrInvalidReadPointer is the error returned when the read pointer is
	// in an invalid position.
	ErrInvalidReadPointer = syscall.EADDRNOTAVAIL
)

// Journal is a handle to an open journald journal.
type Journal struct {
	j      *C.sd_journal
	noCopy noCopy //nolint:unused // Exists only to mark values uncopyable for `go vet`.
}

// Open opens the log journal for reading.
//
// The returned Journal value may only be used from the same operating system
// thread which Open was called from. Using it from only a single goroutine is
// not sufficient; runtime.LockOSThread must also be used.
func Open(flags int) (*Journal, error) {
	j := &Journal{}
	if rc := C.sd_journal_open(&j.j, C.int(flags)); rc != 0 {
		return nil, fmt.Errorf("journald: error opening journal: %w", syscall.Errno(-rc))
	}
	runtime.SetFinalizer(j, (*Journal).Close)
	return j, nil
}

// OpenDir opens the journal files at the specified absolute directory path for
// reading.
//
// The returned Journal value may only be used from the same operating system
// thread which Open was called from. Using it from only a single goroutine is
// not sufficient; runtime.LockOSThread must also be used.
func OpenDir(path string, flags int) (*Journal, error) {
	j := &Journal{}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	if rc := C.sd_journal_open_directory(&j.j, cpath, C.int(flags)); rc != 0 {
		return nil, fmt.Errorf("journald: error opening journal: %w", syscall.Errno(-rc))
	}
	runtime.SetFinalizer(j, (*Journal).Close)
	return j, nil
}

// Close closes the journal. The return value is always nil.
func (j *Journal) Close() error {
	if j.j != nil {
		C.sd_journal_close(j.j)
		runtime.SetFinalizer(j, nil)
		j.j = nil
	}
	return nil
}

// Process processes journal events.
//
// https://www.freedesktop.org/software/systemd/man/sd_journal_process.html
func (j *Journal) Process() (Status, error) {
	s := C.sd_journal_process(j.j)
	if s < 0 {
		return 0, fmt.Errorf("journald: error processing events: %w", syscall.Errno(-s))
	}
	return Status(s), nil
}

// InitializeInotify sets up change notifications for the journal.
func (j *Journal) InitializeInotify() error {
	if rc := C.sd_journal_get_fd(j.j); rc < 0 {
		return fmt.Errorf("journald: error initializing inotify watches: %w", syscall.Errno(-rc))
	}
	return nil
}

// AddMatch adds a match by which to filter the entries of the journal file.
//
// https://www.freedesktop.org/software/systemd/man/sd_journal_add_match.html
func (j *Journal) AddMatch(field, value string) error {
	m := field + "=" + value
	if rc := C.add_match(j.j, m); rc != 0 {
		return fmt.Errorf("journald: error adding match %q: %w", m, syscall.Errno(-rc))
	}
	return nil
}

// Next advances the read pointer to the next entry.
func (j *Journal) Next() (bool, error) {
	rc := C.sd_journal_next(j.j)
	if rc < 0 {
		return false, fmt.Errorf("journald: error advancing read pointer: %w", syscall.Errno(-rc))
	}
	return rc > 0, nil
}

// Previous sets back the read pointer to the previous entry.
func (j *Journal) Previous() (bool, error) {
	rc := C.sd_journal_previous(j.j)
	if rc < 0 {
		return false, fmt.Errorf("journald: error setting back read pointer: %w", syscall.Errno(-rc))
	}
	return rc > 0, nil
}

// PreviousSkip sets back the read pointer by skip entries, returning the number
// of entries set back. skip must be less than or equal to 2147483647
// (2**31 - 1).
//
// skip == 0 is a special case: PreviousSkip(0) resolves the read pointer to a
// discrete position without setting it back to a different entry. The trouble
// is, it always returns zero on recent libsystemd versions. There is no way to
// tell from the return values whether or not it successfully resolved the read
// pointer to a discrete entry.
// https://github.com/systemd/systemd/pull/5930#issuecomment-300878104
func (j *Journal) PreviousSkip(skip uint) (int, error) {
	rc := C.sd_journal_previous_skip(j.j, C.uint64_t(skip))
	if rc < 0 {
		return 0, fmt.Errorf("journald: error setting back read pointer: %w", syscall.Errno(-rc))
	}
	return int(rc), nil
}

// SeekHead sets the read pointer to the position before the oldest available entry.
//
// BUG: SeekHead() followed by Previous() has unexpected behavior.
// https://github.com/systemd/systemd/issues/17662
func (j *Journal) SeekHead() error {
	if rc := C.sd_journal_seek_head(j.j); rc != 0 {
		return fmt.Errorf("journald: error seeking to head of journal: %w", syscall.Errno(-rc))
	}
	return nil
}

// SeekTail sets the read pointer to the position after the most recent available entry.
//
// BUG: SeekTail() followed by Next() has unexpected behavior.
// https://github.com/systemd/systemd/issues/9934
func (j *Journal) SeekTail() error {
	if rc := C.sd_journal_seek_tail(j.j); rc != 0 {
		return fmt.Errorf("journald: error seeking to tail of journal: %w", syscall.Errno(-rc))
	}
	return nil
}

// SeekRealtime seeks to a position with a realtime (wallclock) timestamp after t.
//
// Note that the realtime clock is not necessarily monotonic. If a realtime
// timestamp is ambiguous, the position seeked to is not defined.
func (j *Journal) SeekRealtime(t time.Time) error {
	if rc := C.sd_journal_seek_realtime_usec(j.j, C.uint64_t(t.UnixMicro())); rc != 0 {
		return fmt.Errorf("journald: error seeking to time %v: %w", t, syscall.Errno(-rc))
	}
	return nil
}

// Wait blocks until the journal gets changed or timeout has elapsed.
// Pass a negative timeout to wait indefinitely.
func (j *Journal) Wait(timeout time.Duration) (Status, error) {
	var dur C.uint64_t
	if timeout < 0 {
		// Wait indefinitely.
		dur = ^C.uint64_t(0) // (uint64_t) -1
	} else {
		dur = C.uint64_t(timeout.Microseconds())
	}
	s := C.sd_journal_wait(j.j, dur)
	if s < 0 {
		return 0, fmt.Errorf("journald: error waiting for event: %w", syscall.Errno(-s))
	}
	return Status(s), nil
}

// Realtime returns the realtime timestamp of the current journal entry.
func (j *Journal) Realtime() (time.Time, error) {
	var stamp C.uint64_t
	if rc := C.sd_journal_get_realtime_usec(j.j, &stamp); rc != 0 {
		return time.Time{}, fmt.Errorf("journald: error getting journal entry timestamp: %w", syscall.Errno(-rc))
	}
	return time.UnixMicro(int64(stamp)), nil
}

// Data returns all data fields for the current journal entry.
func (j *Journal) Data() (map[string]string, error) {
	// Copying all the data fields for the entry into a map is more optimal
	// than you might think. Doing so has time complexity O(N), where N is
	// the number of fields in the entry. Looking up a data field in the map
	// is amortized O(1), so the total complexity to look up M data fields
	// is O(N+M). By comparison, looking up a data field using the
	// sd_journal_get_data function has time complexity of O(N) as it is
	// implemented as a linear search through the entry's fields. Therefore
	// looking up M data fields in an entry by calling sd_journal_get_data
	// in a loop would have time complexity of O(N*M).

	m := make(map[string]string)
	j.restartData()
	for {
		var (
			data unsafe.Pointer
			len  C.size_t
		)
		rc := C.sd_journal_enumerate_data(j.j, &data, &len)
		if rc == 0 {
			return m, nil
		} else if rc < 0 {
			return m, fmt.Errorf("journald: error enumerating entry data: %w", syscall.Errno(-rc))
		}

		k, v, _ := strings.Cut(C.GoStringN((*C.char)(data), C.int(len)), "=")
		m[k] = v
	}
}

func (j *Journal) restartData() {
	C.sd_journal_restart_data(j.j)
}

// SetDataThreshold may be used to change the data field size threshold for data
// returned by j.Data(). The threshold is a hint only; larger data fields might
// still be returned.
//
// The default threshold is 64K. To retrieve the complete data fields this
// threshold should be turned off by setting it to 0.
//
// https://www.freedesktop.org/software/systemd/man/sd_journal_set_data_threshold.html
func (j *Journal) SetDataThreshold(v uint) error {
	if rc := C.sd_journal_set_data_threshold(j.j, C.size_t(v)); rc != 0 {
		return fmt.Errorf("journald: error setting journal data threshold: %w", syscall.Errno(-rc))
	}
	return nil
}

type noCopy struct{}

func (noCopy) Lock()   {}
func (noCopy) Unlock() {}
