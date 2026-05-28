package wasip1

import (
	"fmt"

	"github.com/tetratelabs/wazero/experimental/sys"
)

// Errno is neither uint16 nor an alias for parity with wasm.ValueType.
type Errno = uint32

// ErrnoName returns the POSIX error code name, except ErrnoSuccess, which is
// not an error. e.g. Errno2big -> "E2BIG"
func ErrnoName(errno uint32) string {
	if int(errno) < len(errnoToString) {
		return errnoToString[errno]
	}
	return fmt.Sprintf("errno(%d)", errno)
}

// Note: Below prefers POSIX symbol names over WASI ones, even if the docs are from WASI.
// See https://linux.die.net/man/3/errno
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#variants-1
const (
	// ErrnoSuccess No error occurred. System call completed successfully.
	ErrnoSuccess Errno = iota
	// Errno2big Argument list too long.
	Errno2big
	// ErrnoAcces Permission denied.
	ErrnoAcces
	// ErrnoAddrinuse Address in use.
	ErrnoAddrinuse
	// ErrnoAddrnotavail Address not available.
	ErrnoAddrnotavail
	// ErrnoAfnosupport Address family not supported.
	ErrnoAfnosupport
	// ErrnoAgain Resource unavailable, or operation would block.
	ErrnoAgain
	// ErrnoAlready Connection already in progress.
	ErrnoAlready
	// ErrnoBadf Bad file descriptor.
	ErrnoBadf
	// ErrnoBadmsg Bad message.
	ErrnoBadmsg
	// ErrnoBusy Device or resource busy.
	ErrnoBusy
	// ErrnoCanceled Operation canceled.
	ErrnoCanceled
	// ErrnoChild No child processes.
	ErrnoChild
	// ErrnoConnaborted Connection aborted.
	ErrnoConnaborted
	// ErrnoConnrefused Connection refused.
	ErrnoConnrefused
	// ErrnoConnreset Connection reset.
	ErrnoConnreset
	// ErrnoDeadlk Resource deadlock would occur.
	ErrnoDeadlk
	// ErrnoDestaddrreq Destination address required.
	ErrnoDestaddrreq
	// ErrnoDom Mathematics argument out of domain of function.
	ErrnoDom
	// ErrnoDquot Reserved.
	ErrnoDquot
	// ErrnoExist File exists.
	ErrnoExist
	// ErrnoFault Bad address.
	ErrnoFault
	// ErrnoFbig File too large.
	ErrnoFbig
	// ErrnoHostunreach Host is unreachable.
	ErrnoHostunreach
	// ErrnoIdrm Identifier removed.
	ErrnoIdrm
	// ErrnoIlseq Illegal byte sequence.
	ErrnoIlseq
	// ErrnoInprogress Operation in progress.
	ErrnoInprogress
	// ErrnoIntr Interrupted function.
	ErrnoIntr
	// ErrnoInval Invalid argument.
	ErrnoInval
	// ErrnoIo I/O error.
	ErrnoIo
	// ErrnoIsconn Socket is connected.
	ErrnoIsconn
	// ErrnoIsdir Is a directory.
	ErrnoIsdir
	// ErrnoLoop Too many levels of symbolic links.
	ErrnoLoop
	// ErrnoMfile File descriptor value too large.
	ErrnoMfile
	// ErrnoMlink Too many links.
	ErrnoMlink
	// ErrnoMsgsize Message too large.
	ErrnoMsgsize
	// ErrnoMultihop Reserved.
	ErrnoMultihop
	// ErrnoNametoolong Filename too long.
	ErrnoNametoolong
	// ErrnoNetdown Network is down.
	ErrnoNetdown
	// ErrnoNetreset Connection aborted by network.
	ErrnoNetreset
	// ErrnoNetunreach Network unreachable.
	ErrnoNetunreach
	// ErrnoNfile Too many files open in system.
	ErrnoNfile
	// ErrnoNobufs No buffer space available.
	ErrnoNobufs
	// ErrnoNodev No such device.
	ErrnoNodev
	// ErrnoNoent No such file or directory.
	ErrnoNoent
	// ErrnoNoexec Executable file format error.
	ErrnoNoexec
	// ErrnoNolck No locks available.
	ErrnoNolck
	// ErrnoNolink Reserved.
	ErrnoNolink
	// ErrnoNomem Not enough space.
	ErrnoNomem
	// ErrnoNomsg No message of the desired type.
	ErrnoNomsg
	// ErrnoNoprotoopt No message of the desired type.
	ErrnoNoprotoopt
	// ErrnoNospc No space left on device.
	ErrnoNospc
	// ErrnoNosys function not supported.
	ErrnoNosys
	// ErrnoNotconn The socket is not connected.
	ErrnoNotconn
	// ErrnoNotdir Not a directory or a symbolic link to a directory.
	ErrnoNotdir
	// ErrnoNotempty Directory not empty.
	ErrnoNotempty
	// ErrnoNotrecoverable State not recoverable.
	ErrnoNotrecoverable
	// ErrnoNotsock Not a socket.
	ErrnoNotsock
	// ErrnoNotsup Not supported, or operation not supported on socket.
	ErrnoNotsup
	// ErrnoNotty Inappropriate I/O control operation.
	ErrnoNotty
	// ErrnoNxio No such device or address.
	ErrnoNxio
	// ErrnoOverflow Value too large to be stored in data type.
	ErrnoOverflow
	// ErrnoOwnerdead Previous owner died.
	ErrnoOwnerdead
	// ErrnoPerm Operation not permitted.
	ErrnoPerm
	// ErrnoPipe Broken pipe.
	ErrnoPipe
	// ErrnoProto Protocol error.
	ErrnoProto
	// ErrnoProtonosupport Protocol error.
	ErrnoProtonosupport
	// ErrnoPrototype Protocol wrong type for socket.
	ErrnoPrototype
	// ErrnoRange Result too large.
	ErrnoRange
	// ErrnoRofs Read-only file system.
	ErrnoRofs
	// ErrnoSpipe Invalid seek.
	ErrnoSpipe
	// ErrnoSrch No such process.
	ErrnoSrch
	// ErrnoStale Reserved.
	ErrnoStale
	// ErrnoTimedout Connection timed out.
	ErrnoTimedout
	// ErrnoTxtbsy Text file busy.
	ErrnoTxtbsy
	// ErrnoXdev Cross-device link.
	ErrnoXdev

	// Note: ErrnoNotcapable was removed by WASI maintainers.
	// See https://github.com/WebAssembly/wasi-libc/pull/294
)

var errnoToString = [...]string{
	"ESUCCESS",
	"E2BIG",
	"EACCES",
	"EADDRINUSE",
	"EADDRNOTAVAIL",
	"EAFNOSUPPORT",
	"EAGAIN",
	"EALREADY",
	"EBADF",
	"EBADMSG",
	"EBUSY",
	"ECANCELED",
	"ECHILD",
	"ECONNABORTED",
	"ECONNREFUSED",
	"ECONNRESET",
	"EDEADLK",
	"EDESTADDRREQ",
	"EDOM",
	"EDQUOT",
	"EEXIST",
	"EFAULT",
	"EFBIG",
	"EHOSTUNREACH",
	"EIDRM",
	"EILSEQ",
	"EINPROGRESS",
	"EINTR",
	"EINVAL",
	"EIO",
	"EISCONN",
	"EISDIR",
	"ELOOP",
	"EMFILE",
	"EMLINK",
	"EMSGSIZE",
	"EMULTIHOP",
	"ENAMETOOLONG",
	"ENETDOWN",
	"ENETRESET",
	"ENETUNREACH",
	"ENFILE",
	"ENOBUFS",
	"ENODEV",
	"ENOENT",
	"ENOEXEC",
	"ENOLCK",
	"ENOLINK",
	"ENOMEM",
	"ENOMSG",
	"ENOPROTOOPT",
	"ENOSPC",
	"ENOSYS",
	"ENOTCONN",
	"ENOTDIR",
	"ENOTEMPTY",
	"ENOTRECOVERABLE",
	"ENOTSOCK",
	"ENOTSUP",
	"ENOTTY",
	"ENXIO",
	"EOVERFLOW",
	"EOWNERDEAD",
	"EPERM",
	"EPIPE",
	"EPROTO",
	"EPROTONOSUPPORT",
	"EPROTOTYPE",
	"ERANGE",
	"EROFS",
	"ESPIPE",
	"ESRCH",
	"ESTALE",
	"ETIMEDOUT",
	"ETXTBSY",
	"EXDEV",
	"ENOTCAPABLE",
}

// ToErrno coerces the error to a WASI Errno.
//
// Note: Coercion isn't centralized in sys.FSContext because ABI use different
// error codes. For example, wasi-filesystem doesn't map to these
// Errno.
func ToErrno(errno sys.Errno) Errno {
	switch errno {
	case 0:
		return ErrnoSuccess
	case sys.EACCES:
		return ErrnoAcces
	case sys.EAGAIN:
		return ErrnoAgain
	case sys.EBADF:
		return ErrnoBadf
	case sys.EEXIST:
		return ErrnoExist
	case sys.EFAULT:
		return ErrnoFault
	case sys.EINTR:
		return ErrnoIntr
	case sys.EINVAL:
		return ErrnoInval
	case sys.EIO:
		return ErrnoIo
	case sys.EISDIR:
		return ErrnoIsdir
	case sys.ELOOP:
		return ErrnoLoop
	case sys.ENAMETOOLONG:
		return ErrnoNametoolong
	case sys.ENOENT:
		return ErrnoNoent
	case sys.ENOSYS:
		return ErrnoNosys
	case sys.ENOTDIR:
		return ErrnoNotdir
	case sys.ERANGE:
		return ErrnoRange
	case sys.ENOTEMPTY:
		return ErrnoNotempty
	case sys.ENOTSOCK:
		return ErrnoNotsock
	case sys.ENOTSUP:
		return ErrnoNotsup
	case sys.EPERM:
		return ErrnoPerm
	case sys.EROFS:
		return ErrnoRofs
	default:
		return ErrnoIo
	}
}
