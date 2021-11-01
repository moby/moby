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
}
