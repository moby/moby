//go:build !windows

package sys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/cilium/ebpf/internal/unix"
)

// BPF wraps SYS_BPF.
//
// Any pointers contained in attr must use the Pointer type from this package.
func BPF(cmd Cmd, attr unsafe.Pointer, size uintptr) (uintptr, error) {
	// Prevent the Go profiler from repeatedly interrupting the verifier,
	// which could otherwise lead to a livelock due to receiving EAGAIN.
	if cmd == BPF_PROG_LOAD || cmd == BPF_PROG_RUN {
		maskProfilerSignal()
		defer unmaskProfilerSignal()
	}

	tok, err := tokenAttr(cmd, attr)
	if err != nil {
		return 0, fmt.Errorf("apply token attributes: %w", err)
	}

	for {
		r1, _, errNo := unix.Syscall(unix.SYS_BPF, uintptr(cmd), uintptr(attr), size)
		runtime.KeepAlive(attr)

		// As of ~4.20 the verifier can be interrupted by a signal,
		// and returns EAGAIN in that case.
		if errNo == unix.EAGAIN && cmd == BPF_PROG_LOAD {
			continue
		}

		var err error
		if errNo != 0 {
			err = wrappedErrno{errNo}
		}

		// Tokens are in use, attempt to enrich EPERM with capability information.
		if tok != nil && errors.Is(err, unix.EPERM) {
			// Kernels before 6.17 don't have token info, so we can't return accurate
			// errors. Make the token error a hint by adding a question mark.
			//
			// Returning ErrTokenCapabilities here isn't ideal since it's not
			// conclusive, but since this behaviour is limited to one LTS release
			// (6.12), and only when tokens are enabled, it's better than having to
			// account for it in all existing feature probes and tests.
			if tok.info == nil {
				return r1, fmt.Errorf("%w (%w?)", err, ErrTokenCapabilities)
			}

			if terr := tok.Capable(cmd, attr); terr != nil {
				return r1, fmt.Errorf("%w: %w", terr, err)
			}
		}

		return r1, err
	}
}

// ObjGetTyped wraps [ObjGet] with a readlink call to extract the type of the
// underlying bpf object.
func ObjGetTyped(attr *ObjGetAttr) (*FD, ObjType, error) {
	fd, err := ObjGet(attr)
	if err != nil {
		return nil, 0, err
	}

	typ, err := readType(fd)
	if err != nil {
		_ = fd.Close()
		return nil, 0, fmt.Errorf("reading fd type: %w", err)
	}

	return fd, typ, nil
}

// readType returns the bpf object type of the file descriptor by calling
// readlink(3). Returns an error if the file descriptor does not represent a bpf
// object.
func readType(fd *FD) (ObjType, error) {
	s, err := os.Readlink(filepath.Join("/proc/self/fd/", fd.String()))
	if err != nil {
		return 0, fmt.Errorf("readlink fd %d: %w", fd.Int(), err)
	}

	s = strings.TrimPrefix(s, "anon_inode:")

	switch s {
	case "bpf-map":
		return BPF_TYPE_MAP, nil
	case "bpf-prog":
		return BPF_TYPE_PROG, nil
	case "bpf-link":
		return BPF_TYPE_LINK, nil
	}

	return 0, fmt.Errorf("unknown type %s of fd %d", s, fd.Int())
}
