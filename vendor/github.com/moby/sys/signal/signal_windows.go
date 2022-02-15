package signal

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// Signals used in cli/command (no windows equivalent, use
// invalid signals so they don't get handled)
const (
	SIGCHLD  = syscall.Signal(0xff)
	SIGWINCH = syscall.Signal(0xff)
	SIGPIPE  = syscall.Signal(0xff)
)

// SignalMap is a map of "supported" signals. As per the comment in GOLang's
// ztypes_windows.go: "More invented values for signals". Windows doesn't
// really support signals in any way, shape or form that Unix does.
var SignalMap = map[string]syscall.Signal{
	"ABRT": syscall.Signal(windows.SIGABRT),
	"ALRM": syscall.Signal(windows.SIGALRM),
	"BUS":  syscall.Signal(windows.SIGBUS),
	"FPE":  syscall.Signal(windows.SIGFPE),
	"HUP":  syscall.Signal(windows.SIGHUP),
	"ILL":  syscall.Signal(windows.SIGILL),
	"INT":  syscall.Signal(windows.SIGINT),
	"KILL": syscall.Signal(windows.SIGKILL),
	"PIPE": syscall.Signal(windows.SIGPIPE),
	"QUIT": syscall.Signal(windows.SIGQUIT),
	"SEGV": syscall.Signal(windows.SIGSEGV),
	"TERM": syscall.Signal(windows.SIGTERM),
	"TRAP": syscall.Signal(windows.SIGTRAP),

	// additional linux signals supported for LCOW
	"CHLD":   syscall.Signal(0x11),
	"CLD":    syscall.Signal(0x11),
	"CONT":   syscall.Signal(0x12),
	"IO":     syscall.Signal(0x1d),
	"IOT":    syscall.Signal(0x6),
	"POLL":   syscall.Signal(0x1d),
	"PROF":   syscall.Signal(0x1b),
	"PWR":    syscall.Signal(0x1e),
	"STKFLT": syscall.Signal(0x10),
	"STOP":   syscall.Signal(0x13),
	"SYS":    syscall.Signal(0x1f),
	"TSTP":   syscall.Signal(0x14),
	"TTIN":   syscall.Signal(0x15),
	"TTOU":   syscall.Signal(0x16),
	"URG":    syscall.Signal(0x17),
	"USR1":   syscall.Signal(0xa),
	"USR2":   syscall.Signal(0xc),
	"VTALRM": syscall.Signal(0x1a),
	"WINCH":  syscall.Signal(0x1c),
	"XCPU":   syscall.Signal(0x18),
	"XFSZ":   syscall.Signal(0x19),
}
