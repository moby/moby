package system

import (
	"io/ioutil"
	"strconv"
	"syscall"
)

// Works similarly to OpenBSD's "closefrom(2)":
//   The closefrom() call deletes all descriptors numbered fd and higher from
//   the per-process file descriptor table.  It is effectively the same as
//   calling close(2) on each descriptor.
// http://www.openbsd.org/cgi-bin/man.cgi?query=closefrom&sektion=2
//
// See also http://stackoverflow.com/a/918469/433558
func CloseFdsFrom(minFd int) error {
	fdList, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return err
	}
	for _, fi := range fdList {
		fd, err := strconv.Atoi(fi.Name())
		if err != nil {
			// ignore non-numeric file names
			continue
		}

		if fd < minFd {
			// ignore descriptors lower than our specified minimum
			continue
		}

		// intentionally ignore errors from syscall.Close
		syscall.Close(fd)
		// the cases where this might fail are basically file descriptors that have already been closed (including and especially the one that was created when ioutil.ReadDir did the "opendir" syscall)
	}
	return nil
}
