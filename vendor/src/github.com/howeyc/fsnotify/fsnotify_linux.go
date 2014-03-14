// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package fsnotify

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	// Options for inotify_init() are not exported
	// sys_IN_CLOEXEC    uint32 = syscall.IN_CLOEXEC
	// sys_IN_NONBLOCK   uint32 = syscall.IN_NONBLOCK

	// Options for AddWatch
	sys_IN_DONT_FOLLOW uint32 = syscall.IN_DONT_FOLLOW
	sys_IN_ONESHOT     uint32 = syscall.IN_ONESHOT
	sys_IN_ONLYDIR     uint32 = syscall.IN_ONLYDIR

	// The "sys_IN_MASK_ADD" option is not exported, as AddWatch
	// adds it automatically, if there is already a watch for the given path
	// sys_IN_MASK_ADD      uint32 = syscall.IN_MASK_ADD

	// Events
	sys_IN_ACCESS        uint32 = syscall.IN_ACCESS
	sys_IN_ALL_EVENTS    uint32 = syscall.IN_ALL_EVENTS
	sys_IN_ATTRIB        uint32 = syscall.IN_ATTRIB
	sys_IN_CLOSE         uint32 = syscall.IN_CLOSE
	sys_IN_CLOSE_NOWRITE uint32 = syscall.IN_CLOSE_NOWRITE
	sys_IN_CLOSE_WRITE   uint32 = syscall.IN_CLOSE_WRITE
	sys_IN_CREATE        uint32 = syscall.IN_CREATE
	sys_IN_DELETE        uint32 = syscall.IN_DELETE
	sys_IN_DELETE_SELF   uint32 = syscall.IN_DELETE_SELF
	sys_IN_MODIFY        uint32 = syscall.IN_MODIFY
	sys_IN_MOVE          uint32 = syscall.IN_MOVE
	sys_IN_MOVED_FROM    uint32 = syscall.IN_MOVED_FROM
	sys_IN_MOVED_TO      uint32 = syscall.IN_MOVED_TO
	sys_IN_MOVE_SELF     uint32 = syscall.IN_MOVE_SELF
	sys_IN_OPEN          uint32 = syscall.IN_OPEN

	sys_AGNOSTIC_EVENTS = sys_IN_MOVED_TO | sys_IN_MOVED_FROM | sys_IN_CREATE | sys_IN_ATTRIB | sys_IN_MODIFY | sys_IN_MOVE_SELF | sys_IN_DELETE | sys_IN_DELETE_SELF

	// Special events
	sys_IN_ISDIR      uint32 = syscall.IN_ISDIR
	sys_IN_IGNORED    uint32 = syscall.IN_IGNORED
	sys_IN_Q_OVERFLOW uint32 = syscall.IN_Q_OVERFLOW
	sys_IN_UNMOUNT    uint32 = syscall.IN_UNMOUNT
)

type FileEvent struct {
	mask   uint32 // Mask of events
	cookie uint32 // Unique cookie associating related events (for rename(2))
	Name   string // File name (optional)
}

// IsCreate reports whether the FileEvent was triggered by a creation
func (e *FileEvent) IsCreate() bool {
	return (e.mask&sys_IN_CREATE) == sys_IN_CREATE || (e.mask&sys_IN_MOVED_TO) == sys_IN_MOVED_TO
}

// IsDelete reports whether the FileEvent was triggered by a delete
func (e *FileEvent) IsDelete() bool {
	return (e.mask&sys_IN_DELETE_SELF) == sys_IN_DELETE_SELF || (e.mask&sys_IN_DELETE) == sys_IN_DELETE
}

// IsModify reports whether the FileEvent was triggered by a file modification or attribute change
func (e *FileEvent) IsModify() bool {
	return ((e.mask&sys_IN_MODIFY) == sys_IN_MODIFY || (e.mask&sys_IN_ATTRIB) == sys_IN_ATTRIB)
}

// IsRename reports whether the FileEvent was triggered by a change name
func (e *FileEvent) IsRename() bool {
	return ((e.mask&sys_IN_MOVE_SELF) == sys_IN_MOVE_SELF || (e.mask&sys_IN_MOVED_FROM) == sys_IN_MOVED_FROM)
}

// IsAttrib reports whether the FileEvent was triggered by a change in the file metadata.
func (e *FileEvent) IsAttrib() bool {
	return (e.mask & sys_IN_ATTRIB) == sys_IN_ATTRIB
}

type watch struct {
	wd    uint32 // Watch descriptor (as returned by the inotify_add_watch() syscall)
	flags uint32 // inotify flags of this watch (see inotify(7) for the list of valid flags)
}

type Watcher struct {
	mu            sync.Mutex        // Map access
	fd            int               // File descriptor (as returned by the inotify_init() syscall)
	watches       map[string]*watch // Map of inotify watches (key: path)
	fsnFlags      map[string]uint32 // Map of watched files to flags used for filter
	fsnmut        sync.Mutex        // Protects access to fsnFlags.
	paths         map[int]string    // Map of watched paths (key: watch descriptor)
	Error         chan error        // Errors are sent on this channel
	internalEvent chan *FileEvent   // Events are queued on this channel
	Event         chan *FileEvent   // Events are returned on this channel
	done          chan bool         // Channel for sending a "quit message" to the reader goroutine
	isClosed      bool              // Set to true when Close() is first called
}

// NewWatcher creates and returns a new inotify instance using inotify_init(2)
func NewWatcher() (*Watcher, error) {
	fd, errno := syscall.InotifyInit()
	if fd == -1 {
		return nil, os.NewSyscallError("inotify_init", errno)
	}
	w := &Watcher{
		fd:            fd,
		watches:       make(map[string]*watch),
		fsnFlags:      make(map[string]uint32),
		paths:         make(map[int]string),
		internalEvent: make(chan *FileEvent),
		Event:         make(chan *FileEvent),
		Error:         make(chan error),
		done:          make(chan bool, 1),
	}

	go w.readEvents()
	go w.purgeEvents()
	return w, nil
}

// Close closes an inotify watcher instance
// It sends a message to the reader goroutine to quit and removes all watches
// associated with the inotify instance
func (w *Watcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true

	// Remove all watches
	for path := range w.watches {
		w.RemoveWatch(path)
	}

	// Send "quit" message to the reader goroutine
	w.done <- true

	return nil
}

// AddWatch adds path to the watched file set.
// The flags are interpreted as described in inotify_add_watch(2).
func (w *Watcher) addWatch(path string, flags uint32) error {
	if w.isClosed {
		return errors.New("inotify instance already closed")
	}

	w.mu.Lock()
	watchEntry, found := w.watches[path]
	w.mu.Unlock()
	if found {
		watchEntry.flags |= flags
		flags |= syscall.IN_MASK_ADD
	}
	wd, errno := syscall.InotifyAddWatch(w.fd, path, flags)
	if wd == -1 {
		return errno
	}

	w.mu.Lock()
	w.watches[path] = &watch{wd: uint32(wd), flags: flags}
	w.paths[wd] = path
	w.mu.Unlock()

	return nil
}

// Watch adds path to the watched file set, watching all events.
func (w *Watcher) watch(path string) error {
	return w.addWatch(path, sys_AGNOSTIC_EVENTS)
}

// RemoveWatch removes path from the watched file set.
func (w *Watcher) removeWatch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	watch, ok := w.watches[path]
	if !ok {
		return errors.New(fmt.Sprintf("can't remove non-existent inotify watch for: %s", path))
	}
	success, errno := syscall.InotifyRmWatch(w.fd, watch.wd)
	if success == -1 {
		return os.NewSyscallError("inotify_rm_watch", errno)
	}
	delete(w.watches, path)
	return nil
}

// readEvents reads from the inotify file descriptor, converts the
// received events into Event objects and sends them via the Event channel
func (w *Watcher) readEvents() {
	var (
		buf   [syscall.SizeofInotifyEvent * 4096]byte // Buffer for a maximum of 4096 raw events
		n     int                                     // Number of bytes read with read()
		errno error                                   // Syscall errno
	)

	for {
		// See if there is a message on the "done" channel
		select {
		case <-w.done:
			syscall.Close(w.fd)
			close(w.internalEvent)
			close(w.Error)
			return
		default:
		}

		n, errno = syscall.Read(w.fd, buf[:])

		// If EOF is received
		if n == 0 {
			syscall.Close(w.fd)
			close(w.internalEvent)
			close(w.Error)
			return
		}

		if n < 0 {
			w.Error <- os.NewSyscallError("read", errno)
			continue
		}
		if n < syscall.SizeofInotifyEvent {
			w.Error <- errors.New("inotify: short read in readEvents()")
			continue
		}

		var offset uint32 = 0
		// We don't know how many events we just read into the buffer
		// While the offset points to at least one whole event...
		for offset <= uint32(n-syscall.SizeofInotifyEvent) {
			// Point "raw" to the event in the buffer
			raw := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			event := new(FileEvent)
			event.mask = uint32(raw.Mask)
			event.cookie = uint32(raw.Cookie)
			nameLen := uint32(raw.Len)
			// If the event happened to the watched directory or the watched file, the kernel
			// doesn't append the filename to the event, but we would like to always fill the
			// the "Name" field with a valid filename. We retrieve the path of the watch from
			// the "paths" map.
			w.mu.Lock()
			event.Name = w.paths[int(raw.Wd)]
			w.mu.Unlock()
			watchedName := event.Name
			if nameLen > 0 {
				// Point "bytes" at the first byte of the filename
				bytes := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				// The filename is padded with NUL bytes. TrimRight() gets rid of those.
				event.Name += "/" + strings.TrimRight(string(bytes[0:nameLen]), "\000")
			}

			// Send the events that are not ignored on the events channel
			if !event.ignoreLinux() {
				// Setup FSNotify flags (inherit from directory watch)
				w.fsnmut.Lock()
				if _, fsnFound := w.fsnFlags[event.Name]; !fsnFound {
					if fsnFlags, watchFound := w.fsnFlags[watchedName]; watchFound {
						w.fsnFlags[event.Name] = fsnFlags
					} else {
						w.fsnFlags[event.Name] = FSN_ALL
					}
				}
				w.fsnmut.Unlock()

				w.internalEvent <- event
			}

			// Move to the next event in the buffer
			offset += syscall.SizeofInotifyEvent + nameLen
		}
	}
}

// Certain types of events can be "ignored" and not sent over the Event
// channel. Such as events marked ignore by the kernel, or MODIFY events
// against files that do not exist.
func (e *FileEvent) ignoreLinux() bool {
	// Ignore anything the inotify API says to ignore
	if e.mask&sys_IN_IGNORED == sys_IN_IGNORED {
		return true
	}

	// If the event is not a DELETE or RENAME, the file must exist.
	// Otherwise the event is ignored.
	// *Note*: this was put in place because it was seen that a MODIFY
	// event was sent after the DELETE. This ignores that MODIFY and
	// assumes a DELETE will come or has come if the file doesn't exist.
	if !(e.IsDelete() || e.IsRename()) {
		_, statErr := os.Lstat(e.Name)
		return os.IsNotExist(statErr)
	}
	return false
}
