package lock

import (
	"errors"
	"github.com/howeyc/fsnotify"
	"io"
	"os"
	"syscall"
)

var ErrAlreadyLocked = errors.New("File already locked")

// Lock the path.
//
// You should always Close the return io.Closer. It'll either unlock the path
// or stop watching for changes.
//
// If the chan returned is not nil, it means that the file is already locker.
// You can just wait for an event on the channel to now when it's unlocked.
func Lock(path string) (io.Closer, chan *fsnotify.FileEvent, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			watch, err := fsnotify.NewWatcher()
			if err != nil {
				return &nopCloser{}, nil, err
			}
			if err = watch.Watch(path); err != nil {
				return &nopCloser{}, nil, err
			}
			return &unWatcher{watch, path}, watch.Event, ErrAlreadyLocked
		} else {
			return &nopCloser{}, nil, err
		}
	}
	return &unLocker{fd, path}, nil, nil
}

type unLocker struct {
	fd   int
	path string
}

func (u *unLocker) Close() error {
	syscall.Close(u.fd)
	return syscall.Unlink(u.path)
}

type unWatcher struct {
	watch *fsnotify.Watcher
	path  string
}

func (u *unWatcher) Close() error {
	return u.watch.RemoveWatch(u.path)
}

type nopCloser struct{}

func (c *nopCloser) Close() error { return nil }
