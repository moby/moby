package ploop

import "fmt"

type Err struct {
	c int
	s string
}

// SYSEXIT_* errors
const (
	_ = iota
	E_CREAT
	E_DEVICE
	E_DEVIOC
	E_OPEN
	E_MALLOC
	E_READ
	E_WRITE
	E_RESERVED_8
	E_SYSFS
	E_RESERVED_10
	E_PLOOPFMT
	E_SYS
	E_PROTOCOL
	E_LOOP
	E_FSTAT
	E_FSYNC
	E_EBUSY
	E_FLOCK
	E_FTRUNCATE
	E_FALLOCATE
	E_MOUNT
	E_UMOUNT
	E_LOCK
	E_MKFS
	E_RESERVED_25
	E_RESIZE_FS
	E_MKDIR
	E_RENAME
	E_ABORT
	E_RELOC
	E_RESERVED_31
	E_RESERVED_32
	E_CHANGE_GPT
	E_RESERVED_34
	E_UNLINK
	E_MKNOD
	E_PLOOPINUSE
	E_PARAM
	E_DISKDESCR
	E_DEV_NOT_MOUNTED
	E_FSCK
	E_RESERVED_42
	E_NOSNAP
)

var ErrCodes = []string{
	E_CREAT:           "E_CREAT",
	E_DEVICE:          "E_DEVICE",
	E_DEVIOC:          "E_DEVIOC",
	E_OPEN:            "E_OPEN",
	E_MALLOC:          "E_MALLOC",
	E_READ:            "E_READ",
	E_WRITE:           "E_WRITE",
	E_RESERVED_8:      "E_RESERVED",
	E_SYSFS:           "E_SYSFS",
	E_RESERVED_10:     "E_RESERVED",
	E_PLOOPFMT:        "E_PLOOPFMT",
	E_SYS:             "E_SYS",
	E_PROTOCOL:        "E_PROTOCOL",
	E_LOOP:            "E_LOOP",
	E_FSTAT:           "E_FSTAT",
	E_FSYNC:           "E_FSYNC",
	E_EBUSY:           "E_EBUSY",
	E_FLOCK:           "E_FLOCK",
	E_FTRUNCATE:       "E_FTRUNCATE",
	E_FALLOCATE:       "E_FALLOCATE",
	E_MOUNT:           "E_MOUNT",
	E_UMOUNT:          "E_UMOUNT",
	E_LOCK:            "E_LOCK",
	E_MKFS:            "E_MKFS",
	E_RESERVED_25:     "E_RESERVED",
	E_RESIZE_FS:       "E_RESIZE_FS",
	E_MKDIR:           "E_MKDIR",
	E_RENAME:          "E_RENAME",
	E_ABORT:           "E_ABORT",
	E_RELOC:           "E_RELOC",
	E_RESERVED_31:     "E_RESERVED",
	E_RESERVED_32:     "E_RESERVED",
	E_CHANGE_GPT:      "E_CHANGE_GPT",
	E_RESERVED_34:     "E_RESERVED",
	E_UNLINK:          "E_UNLINK",
	E_MKNOD:           "E_MKNOD",
	E_PLOOPINUSE:      "E_PLOOPINUSE",
	E_PARAM:           "E_PARAM",
	E_DISKDESCR:       "E_DISKDESCR",
	E_DEV_NOT_MOUNTED: "E_DEV_NOT_MOUNTED",
	E_FSCK:            "E_FSCK",
	E_RESERVED_42:     "E_RESERVED",
	E_NOSNAP:          "E_NOSNAP",
}

// Error returns a string representation of a ploop error
func (e *Err) Error() string {
	s := "E_UNKNOWN"
	if e.c > 0 && e.c < len(ErrCodes) {
		s = ErrCodes[e.c]
	}

	return fmt.Sprintf("ploop error %d (%s): %s", e.c, s, e.s)
}

// IsError checks if an error is a specific ploop error
func IsError(err error, code int) bool {
	perr, ok := err.(*Err)
	return ok && perr.c == code
}

// IsNotMounted returns true if an error is ploop "device is not mounted"
func IsNotMounted(err error) bool {
	return IsError(err, E_DEV_NOT_MOUNTED)
}
