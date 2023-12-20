// Package filelease implements "best effort" file locking, for Linux hosts, by
// wrapping fcntl(2)'s F_SETLEASE.
//
// A file that is bind mounted into a container (for example "/etc/hosts",
// "/etc/resolv.conf") can't be updated by moving a new version into place. So,
// when updating it, it's possible for consumers inside the container to see a
// truncated or incomplete version of the file. If a write lease can be obtained
// for one of these files, processes attempting to open the file in the container
// will block until the lease is dropped.
//
// If the file already has open descriptors, a lease cannot be obtained. OpenFile
// can be configured to retry, and to return an open file descriptor even if the
// lease cannot be acquired.
package filelease

import (
	"context"
	"os"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/types"
	"github.com/pkg/errors"
)

// Use OpenFile to open a file, attempt to acquire a write lease, and
// construct a FileLease.
type FileLease struct {
	*os.File
	leaseErr error
}

// Opts describes options for NewFileLeaser.
type Opts struct {
	flags    int           // flag for os.OpenFile()
	mode     os.FileMode   // perm for os.OpenFile()
	attempts int           // Max attempts to acquire lease.
	interval time.Duration // Interval between attempts to acquire lease.
}

// defaultOpts returns a default set of options that can be modified by the WithXXX() functions below.
func defaultOpts() *Opts {
	return &Opts{
		flags:    os.O_CREATE | os.O_RDWR,
		mode:     0644,
		attempts: 2,
		interval: 10 * time.Millisecond,
	}
}

// WithFlags can be used to modify the flag value passed to os.OpenFile().
func WithFlags(flags int) func(*Opts) {
	return func(o *Opts) {
		o.flags = flags
	}
}

// WithFileMode can be used to modify the perm value passed to os.OpenFile().
func WithFileMode(mode os.FileMode) func(*Opts) {
	return func(o *Opts) {
		o.mode = mode
	}
}

// WithRetry can be used to modify the number of attempts to make at acquiring
// the write lease, and the interval between attempts, when it fails because the
// file is already open.
func WithRetry(attempts int, interval time.Duration) func(*Opts) {
	return func(o *Opts) {
		o.attempts = attempts
		o.interval = interval
	}
}

// OpenFile opens a file and tries to acquire a write lease on it.
//
// If mustLease is false, the lease is best-effort - if it cannot be acquired (or
// file leases are not implemented for this OS) the open fd is returned without a
// lease. In this case, an error is only returned if the file cannot be opened.
//
// If a lease is acquired, it's automatically released when the file is closed.
// Until then, other readers and writers will block when they try to open the
// file. SIGIO will be delivered to this process when a second reader or writer
// attempts to open the file, it is ignored by default. (The lease is forcibly
// released by the kernel "/proc/sys/fs/lease-break-time" seconds after the
// signal is delivered.)
//
// If there is already an open descriptor for the file, belonging to this or any
// other process, this function will try to obtain the lease Opts.Attempts times,
// at intervals of Opts.Interval.
//
// File leasing is only implemented for Linux. For other OSs, the file is just
// opened normally, and OpenFile with mustLease will always fail.
func OpenFile(path string, mustLease bool, options ...func(*Opts)) (*FileLease, error) {
	opts := defaultOpts()
	for _, opt := range options {
		opt(opts)
	}

	f, err := os.OpenFile(path, opts.flags, opts.mode)
	if err != nil {
		return nil, err
	}
	err = getWriteLease(f, opts)
	if err != nil {
		if mustLease {
			f.Close()
			return nil, err
		}
		// Return the open file without a lease.
		if _, ok := err.(types.NotImplementedError); !ok {
			// File leasing is implemented, so log the conflict.
			log.G(context.TODO()).WithError(err).WithField("path", path).Info("opened without lease")
		}
	}
	return &FileLease{File: f, leaseErr: err}, nil
}

// Leased returns true if the write lease was obtained after the file was opened.
func (fl *FileLease) Leased() bool {
	return fl.leaseErr == nil
}

// WriteFile rewrites an open file by truncating it, then writing b.
func (fl *FileLease) WriteFile(b []byte) error {
	if err := fl.File.Truncate(0); err != nil {
		return errors.Wrap(err, "truncate file for rewrite")
	}
	if _, err := fl.File.Seek(0, 0); err != nil {
		return errors.Wrap(err, "seek to start of file for rewrite")
	}
	_, err := fl.File.Write(b)
	return errors.Wrap(err, "rewrite file")
}
