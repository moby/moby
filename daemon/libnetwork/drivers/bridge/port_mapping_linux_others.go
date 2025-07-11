//go:build linux && !386

package bridge

import (
	"syscall"
	"unsafe"

	"github.com/ishidawataru/sctp"
)

func setSCTPInitMsg(sd int, options sctp.InitMsg) syscall.Errno {
	_, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(sd),
		sctp.SOL_SCTP,
		sctp.SCTP_INITMSG,
		uintptr(unsafe.Pointer(&options)), // #nosec G103 -- Ignore "G103: Use of unsafe calls should be audited"
		unsafe.Sizeof(options),
		0)
	return errno
}
