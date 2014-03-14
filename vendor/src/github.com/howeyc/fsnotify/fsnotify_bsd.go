// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build freebsd openbsd netbsd darwin

package fsnotify

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const (
	// Flags (from <sys/event.h>)
	sys_NOTE_DELETE = 0x0001 /* vnode was removed */
	sys_NOTE_WRITE  = 0x0002 /* data contents changed */
	sys_NOTE_EXTEND = 0x0004 /* size increased */
	sys_NOTE_ATTRIB = 0x0008 /* attributes changed */
	sys_NOTE_LINK   = 0x0010 /* link count changed */
	sys_NOTE_RENAME = 0x0020 /* vnode was renamed */
	sys_NOTE_REVOKE = 0x0040 /* vnode access was revoked */

	// Watch all events
	sys_NOTE_ALLEVENTS = sys_NOTE_DELETE | sys_NOTE_WRITE | sys_NOTE_ATTRIB | sys_NOTE_RENAME

	// Block for 100 ms on each call to kevent
	keventWaitTime = 100e6
)

type FileEvent struct {
	mask   uint32 // Mask of events
	Name   string // File name (optional)
	create bool   // set by fsnotify package if found new file
}

// IsCreate reports whether the FileEvent was triggered by a creation
func (e *FileEvent) IsCreate() bool { return e.create }

// IsDelete reports whether the FileEvent was triggered by a delete
func (e *FileEvent) IsDelete() bool { return (e.mask & sys_NOTE_DELETE) == sys_NOTE_DELETE }

// IsModify reports whether the FileEvent was triggered by a file modification
func (e *FileEvent) IsModify() bool {
	return ((e.mask&sys_NOTE_WRITE) == sys_NOTE_WRITE || (e.mask&sys_NOTE_ATTRIB) == sys_NOTE_ATTRIB)
}

// IsRename reports whether the FileEvent was triggered by a change name
func (e *FileEvent) IsRename() bool { return (e.mask & sys_NOTE_RENAME) == sys_NOTE_RENAME }

// IsAttrib reports whether the FileEvent was triggered by a change in the file metadata.
func (e *FileEvent) IsAttrib() bool {
	return (e.mask & sys_NOTE_ATTRIB) == sys_NOTE_ATTRIB
}

type Watcher struct {
	mu              sync.Mutex          // Mutex for the Watcher itself.
	kq              int                 // File descriptor (as returned by the kqueue() syscall)
	watches         map[string]int      // Map of watched file descriptors (key: path)
	wmut            sync.Mutex          // Protects access to watches.
	fsnFlags        map[string]uint32   // Map of watched files to flags used for filter
	fsnmut          sync.Mutex          // Protects access to fsnFlags.
	enFlags         map[string]uint32   // Map of watched files to evfilt note flags used in kqueue
	enmut           sync.Mutex          // Protects access to enFlags.
	paths           map[int]string      // Map of watched paths (key: watch descriptor)
	finfo           map[int]os.FileInfo // Map of file information (isDir, isReg; key: watch descriptor)
	pmut            sync.Mutex          // Protects access to paths and finfo.
	fileExists      map[string]bool     // Keep track of if we know this file exists (to stop duplicate create events)
	femut           sync.Mutex          // Protects access to fileExists.
	externalWatches map[string]bool     // Map of watches added by user of the library.
	ewmut           sync.Mutex          // Protects access to externalWatches.
	Error           chan error          // Errors are sent on this channel
	internalEvent   chan *FileEvent     // Events are queued on this channel
	Event           chan *FileEvent     // Events are returned on this channel
	done            chan bool           // Channel for sending a "quit message" to the reader goroutine
	isClosed        bool                // Set to true when Close() is first called
	kbuf            [1]syscall.Kevent_t // An event buffer for Add/Remove watch
	bufmut          sync.Mutex          // Protects access to kbuf.
}

// NewWatcher creates and returns a new kevent instance using kqueue(2)
func NewWatcher() (*Watcher, error) {
	fd, errno := syscall.Kqueue()
	if fd == -1 {
		return nil, os.NewSyscallError("kqueue", errno)
	}
	w := &Watcher{
		kq:              fd,
		watches:         make(map[string]int),
		fsnFlags:        make(map[string]uint32),
		enFlags:         make(map[string]uint32),
		paths:           make(map[int]string),
		finfo:           make(map[int]os.FileInfo),
		fileExists:      make(map[string]bool),
		externalWatches: make(map[string]bool),
		internalEvent:   make(chan *FileEvent),
		Event:           make(chan *FileEvent),
		Error:           make(chan error),
		done:            make(chan bool, 1),
	}

	go w.readEvents()
	go w.purgeEvents()
	return w, nil
}

// Close closes a kevent watcher instance
// It sends a message to the reader goroutine to quit and removes all watches
// associated with the kevent instance
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.isClosed {
		w.mu.Unlock()
		return nil
	}
	w.isClosed = true
	w.mu.Unlock()

	// Send "quit" message to the reader goroutine
	w.done <- true
	w.pmut.Lock()
	ws := w.watches
	w.pmut.Unlock()
	for path := range ws {
		w.removeWatch(path)
	}

	return nil
}

// AddWatch adds path to the watched file set.
// The flags are interpreted as described in kevent(2).
func (w *Watcher) addWatch(path string, flags uint32) error {
	w.mu.Lock()
	if w.isClosed {
		w.mu.Unlock()
		return errors.New("kevent instance already closed")
	}
	w.mu.Unlock()

	watchDir := false

	w.wmut.Lock()
	watchfd, found := w.watches[path]
	w.wmut.Unlock()
	if !found {
		fi, errstat := os.Lstat(path)
		if errstat != nil {
			return errstat
		}

		// don't watch socket
		if fi.Mode()&os.ModeSocket == os.ModeSocket {
			return nil
		}

		// Follow Symlinks
		// Unfortunately, Linux can add bogus symlinks to watch list without
		// issue, and Windows can't do symlinks period (AFAIK). To  maintain
		// consistency, we will act like everything is fine. There will simply
		// be no file events for broken symlinks.
		// Hence the returns of nil on errors.
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			path, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}

			fi, errstat = os.Lstat(path)
			if errstat != nil {
				return nil
			}
		}

		fd, errno := syscall.Open(path, open_FLAGS, 0700)
		if fd == -1 {
			return errno
		}
		watchfd = fd

		w.wmut.Lock()
		w.watches[path] = watchfd
		w.wmut.Unlock()

		w.pmut.Lock()
		w.paths[watchfd] = path
		w.finfo[watchfd] = fi
		w.pmut.Unlock()
	}
	// Watch the directory if it has not been watched before.
	w.pmut.Lock()
	w.enmut.Lock()
	if w.finfo[watchfd].IsDir() &&
		(flags&sys_NOTE_WRITE) == sys_NOTE_WRITE &&
		(!found || (w.enFlags[path]&sys_NOTE_WRITE) != sys_NOTE_WRITE) {
		watchDir = true
	}
	w.enmut.Unlock()
	w.pmut.Unlock()

	w.enmut.Lock()
	w.enFlags[path] = flags
	w.enmut.Unlock()

	w.bufmut.Lock()
	watchEntry := &w.kbuf[0]
	watchEntry.Fflags = flags
	syscall.SetKevent(watchEntry, watchfd, syscall.EVFILT_VNODE, syscall.EV_ADD|syscall.EV_CLEAR)
	entryFlags := watchEntry.Flags
	w.bufmut.Unlock()

	wd, errno := syscall.Kevent(w.kq, w.kbuf[:], nil, nil)
	if wd == -1 {
		return errno
	} else if (entryFlags & syscall.EV_ERROR) == syscall.EV_ERROR {
		return errors.New("kevent add error")
	}

	if watchDir {
		errdir := w.watchDirectoryFiles(path)
		if errdir != nil {
			return errdir
		}
	}
	return nil
}

// Watch adds path to the watched file set, watching all events.
func (w *Watcher) watch(path string) error {
	w.ewmut.Lock()
	w.externalWatches[path] = true
	w.ewmut.Unlock()
	return w.addWatch(path, sys_NOTE_ALLEVENTS)
}

// RemoveWatch removes path from the watched file set.
func (w *Watcher) removeWatch(path string) error {
	w.wmut.Lock()
	watchfd, ok := w.watches[path]
	w.wmut.Unlock()
	if !ok {
		return errors.New(fmt.Sprintf("can't remove non-existent kevent watch for: %s", path))
	}
	w.bufmut.Lock()
	watchEntry := &w.kbuf[0]
	syscall.SetKevent(watchEntry, watchfd, syscall.EVFILT_VNODE, syscall.EV_DELETE)
	success, errno := syscall.Kevent(w.kq, w.kbuf[:], nil, nil)
	w.bufmut.Unlock()
	if success == -1 {
		return os.NewSyscallError("kevent_rm_watch", errno)
	} else if (watchEntry.Flags & syscall.EV_ERROR) == syscall.EV_ERROR {
		return errors.New("kevent rm error")
	}
	syscall.Close(watchfd)
	w.wmut.Lock()
	delete(w.watches, path)
	w.wmut.Unlock()
	w.enmut.Lock()
	delete(w.enFlags, path)
	w.enmut.Unlock()
	w.pmut.Lock()
	delete(w.paths, watchfd)
	fInfo := w.finfo[watchfd]
	delete(w.finfo, watchfd)
	w.pmut.Unlock()

	// Find all watched paths that are in this directory that are not external.
	if fInfo.IsDir() {
		var pathsToRemove []string
		w.pmut.Lock()
		for _, wpath := range w.paths {
			wdir, _ := filepath.Split(wpath)
			if filepath.Clean(wdir) == filepath.Clean(path) {
				w.ewmut.Lock()
				if !w.externalWatches[wpath] {
					pathsToRemove = append(pathsToRemove, wpath)
				}
				w.ewmut.Unlock()
			}
		}
		w.pmut.Unlock()
		for _, p := range pathsToRemove {
			// Since these are internal, not much sense in propagating error
			// to the user, as that will just confuse them with an error about
			// a path they did not explicitly watch themselves.
			w.removeWatch(p)
		}
	}

	return nil
}

// readEvents reads from the kqueue file descriptor, converts the
// received events into Event objects and sends them via the Event channel
func (w *Watcher) readEvents() {
	var (
		eventbuf [10]syscall.Kevent_t // Event buffer
		events   []syscall.Kevent_t   // Received events
		twait    *syscall.Timespec    // Time to block waiting for events
		n        int                  // Number of events returned from kevent
		errno    error                // Syscall errno
	)
	events = eventbuf[0:0]
	twait = new(syscall.Timespec)
	*twait = syscall.NsecToTimespec(keventWaitTime)

	for {
		// See if there is a message on the "done" channel
		var done bool
		select {
		case done = <-w.done:
		default:
		}

		// If "done" message is received
		if done {
			errno := syscall.Close(w.kq)
			if errno != nil {
				w.Error <- os.NewSyscallError("close", errno)
			}
			close(w.internalEvent)
			close(w.Error)
			return
		}

		// Get new events
		if len(events) == 0 {
			n, errno = syscall.Kevent(w.kq, nil, eventbuf[:], twait)

			// EINTR is okay, basically the syscall was interrupted before
			// timeout expired.
			if errno != nil && errno != syscall.EINTR {
				w.Error <- os.NewSyscallError("kevent", errno)
				continue
			}

			// Received some events
			if n > 0 {
				events = eventbuf[0:n]
			}
		}

		// Flush the events we received to the events channel
		for len(events) > 0 {
			fileEvent := new(FileEvent)
			watchEvent := &events[0]
			fileEvent.mask = uint32(watchEvent.Fflags)
			w.pmut.Lock()
			fileEvent.Name = w.paths[int(watchEvent.Ident)]
			fileInfo := w.finfo[int(watchEvent.Ident)]
			w.pmut.Unlock()
			if fileInfo != nil && fileInfo.IsDir() && !fileEvent.IsDelete() {
				// Double check to make sure the directory exist. This can happen when
				// we do a rm -fr on a recursively watched folders and we receive a
				// modification event first but the folder has been deleted and later
				// receive the delete event
				if _, err := os.Lstat(fileEvent.Name); os.IsNotExist(err) {
					// mark is as delete event
					fileEvent.mask |= sys_NOTE_DELETE
				}
			}

			if fileInfo != nil && fileInfo.IsDir() && fileEvent.IsModify() && !fileEvent.IsDelete() {
				w.sendDirectoryChangeEvents(fileEvent.Name)
			} else {
				// Send the event on the events channel
				w.internalEvent <- fileEvent
			}

			// Move to next event
			events = events[1:]

			if fileEvent.IsRename() {
				w.removeWatch(fileEvent.Name)
				w.femut.Lock()
				delete(w.fileExists, fileEvent.Name)
				w.femut.Unlock()
			}
			if fileEvent.IsDelete() {
				w.removeWatch(fileEvent.Name)
				w.femut.Lock()
				delete(w.fileExists, fileEvent.Name)
				w.femut.Unlock()

				// Look for a file that may have overwritten this
				// (ie mv f1 f2 will delete f2 then create f2)
				fileDir, _ := filepath.Split(fileEvent.Name)
				fileDir = filepath.Clean(fileDir)
				w.wmut.Lock()
				_, found := w.watches[fileDir]
				w.wmut.Unlock()
				if found {
					// make sure the directory exist before we watch for changes. When we
					// do a recursive watch and perform rm -fr, the parent directory might
					// have gone missing, ignore the missing directory and let the
					// upcoming delete event remove the watch form the parent folder
					if _, err := os.Lstat(fileDir); !os.IsNotExist(err) {
						w.sendDirectoryChangeEvents(fileDir)
					}
				}
			}
		}
	}
}

func (w *Watcher) watchDirectoryFiles(dirPath string) error {
	// Get all files
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Search for new files
	for _, fileInfo := range files {
		filePath := filepath.Join(dirPath, fileInfo.Name())

		// Inherit fsnFlags from parent directory
		w.fsnmut.Lock()
		if flags, found := w.fsnFlags[dirPath]; found {
			w.fsnFlags[filePath] = flags
		} else {
			w.fsnFlags[filePath] = FSN_ALL
		}
		w.fsnmut.Unlock()

		if fileInfo.IsDir() == false {
			// Watch file to mimic linux fsnotify
			e := w.addWatch(filePath, sys_NOTE_ALLEVENTS)
			if e != nil {
				return e
			}
		} else {
			// If the user is currently watching directory
			// we want to preserve the flags used
			w.enmut.Lock()
			currFlags, found := w.enFlags[filePath]
			w.enmut.Unlock()
			var newFlags uint32 = sys_NOTE_DELETE
			if found {
				newFlags |= currFlags
			}

			// Linux gives deletes if not explicitly watching
			e := w.addWatch(filePath, newFlags)
			if e != nil {
				return e
			}
		}
		w.femut.Lock()
		w.fileExists[filePath] = true
		w.femut.Unlock()
	}

	return nil
}

// sendDirectoryEvents searches the directory for newly created files
// and sends them over the event channel. This functionality is to have
// the BSD version of fsnotify match linux fsnotify which provides a
// create event for files created in a watched directory.
func (w *Watcher) sendDirectoryChangeEvents(dirPath string) {
	// Get all files
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		w.Error <- err
	}

	// Search for new files
	for _, fileInfo := range files {
		filePath := filepath.Join(dirPath, fileInfo.Name())
		w.femut.Lock()
		_, doesExist := w.fileExists[filePath]
		w.femut.Unlock()
		if !doesExist {
			// Inherit fsnFlags from parent directory
			w.fsnmut.Lock()
			if flags, found := w.fsnFlags[dirPath]; found {
				w.fsnFlags[filePath] = flags
			} else {
				w.fsnFlags[filePath] = FSN_ALL
			}
			w.fsnmut.Unlock()

			// Send create event
			fileEvent := new(FileEvent)
			fileEvent.Name = filePath
			fileEvent.create = true
			w.internalEvent <- fileEvent
		}
		w.femut.Lock()
		w.fileExists[filePath] = true
		w.femut.Unlock()
	}
	w.watchDirectoryFiles(dirPath)
}
