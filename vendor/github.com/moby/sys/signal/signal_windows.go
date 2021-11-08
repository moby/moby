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
	// DefaultStopSignal is the syscall signal used to stop a container in windows systems.
	DefaultStopSignal = "15"
)

// SignalMap is a map of "supported" signals. As per the comment in GOLang's
// ztypes_windows.go: "More invented values for signals". Windows doesn't
// really support signals in any way, shape or form that Unix does.
var SignalMap = map[string]syscall.Signal{
	"HUP":  syscall.Signal(windows.SIGHUP),
	"INT":  syscall.Signal(windows.SIGINT),
	"QUIT": syscall.Signal(windows.SIGQUIT),
	"ILL":  syscall.Signal(windows.SIGILL),
	"TRAP": syscall.Signal(windows.SIGTRAP),
	"ABRT": syscall.Signal(windows.SIGABRT),
	"BUS":  syscall.Signal(windows.SIGBUS),
	"FPE":  syscall.Signal(windows.SIGFPE),
	"KILL": syscall.Signal(windows.SIGKILL),
	"SEGV": syscall.Signal(windows.SIGSEGV),
	"PIPE": syscall.Signal(windows.SIGPIPE),
	"ALRM": syscall.Signal(windows.SIGALRM),
	"TERM": syscall.Signal(windows.SIGTERM),
}
