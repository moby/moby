package utils

import (
	"os"
	"os/signal"
	"syscall"
)

func init() {
	signalMap = map[syscall.Signal]syscall.Signal{
		syscall.SIGABRT:   0x6,
		syscall.SIGALRM:   0xe,
		syscall.SIGBUS:    0x7,
		syscall.SIGCHLD:   0x11,
		syscall.SIGCONT:   0x12,
		syscall.SIGFPE:    0x8,
		syscall.SIGHUP:    0x1,
		syscall.SIGILL:    0x4,
		syscall.SIGINT:    0x2,
		syscall.SIGIO:     0x1d,
		syscall.SIGKILL:   0x9,
		syscall.SIGPIPE:   0xd,
		syscall.SIGPROF:   0x1b,
		syscall.SIGQUIT:   0x3,
		syscall.SIGSEGV:   0xb,
		syscall.SIGSTOP:   0x13,
		syscall.SIGSYS:    0x1f,
		syscall.SIGTERM:   0xf,
		syscall.SIGTRAP:   0x5,
		syscall.SIGTSTP:   0x14,
		syscall.SIGTTIN:   0x15,
		syscall.SIGTTOU:   0x16,
		syscall.SIGURG:    0x17,
		syscall.SIGUSR1:   0x1a,
		syscall.SIGUSR2:   0x1c,
		syscall.SIGVTALRM: 0x1a,
		syscall.SIGWINCH:  0x1c,
		syscall.SIGXCPU:   0x18,
		syscall.SIGXFSZ:   0x19,
	}
	// SIGINFO is not defined on linux
	// SIGIOT is SIGIO on darwin
	// SIGPWR is not defined on darwin
	// SIGSTKFLT is not defined on darwin
	// SIGTHR not present on darwin
	// SIGUNUSED not defined on darwin
	// SIGEMT is not defined on linux
}

func suspend() error {
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGSTOP); err != nil {
		return err
	}
	return nil
}

func CatchAll(sigc chan os.Signal) {
	signal.Notify(sigc,
		syscall.SIGABRT,
		syscall.SIGALRM,
		syscall.SIGBUS,
		syscall.SIGCHLD,
		syscall.SIGCONT,
		syscall.SIGEMT,
		syscall.SIGFPE,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINFO,
		syscall.SIGINT,
		syscall.SIGIO,
		syscall.SIGIOT,
		syscall.SIGKILL,
		syscall.SIGPIPE,
		syscall.SIGPROF,
		syscall.SIGQUIT,
		syscall.SIGSEGV,
		syscall.SIGSTOP,
		syscall.SIGSYS,
		syscall.SIGTERM,
		syscall.SIGTRAP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
		syscall.SIGURG,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGVTALRM,
		syscall.SIGWINCH,
		syscall.SIGXCPU,
		syscall.SIGXFSZ,
		syscall.SIGTSTP,
	)
}
