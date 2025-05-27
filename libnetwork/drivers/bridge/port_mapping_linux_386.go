package bridge

import (
	"syscall"
	"unsafe"

	"github.com/ishidawataru/sctp"
)

const sysSetsockopt = 14 // See https://elixir.bootlin.com/linux/v6.13.3/source/include/uapi/linux/net.h#L40

func setSCTPInitMsg(sd int, options sctp.InitMsg) syscall.Errno {
	_, _, errno := syscall.Syscall6(syscall.SYS_SOCKETCALL, // See `man 2 socketcall`
		sysSetsockopt,
		uintptr(sd),
		sctp.SOL_SCTP,
		sctp.SCTP_INITMSG,
		uintptr(unsafe.Pointer(&options)), // #nosec G103 -- Ignore "G103: Use of unsafe calls should be audited"
		unsafe.Sizeof(options))
	return errno
}
