package utils

import (
	"os"
	"os/signal"
	"syscall"
)

func CatchAll(sigc chan os.Signal) {
	signal.Notify(sigc,
		syscall.SIGABRT,
		syscall.SIGALRM,
		syscall.SIGBUS,
		syscall.SIGCHLD,
		syscall.SIGCLD,
		syscall.SIGCONT,
		syscall.SIGFPE,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINT,
		syscall.SIGIO,
		syscall.SIGIOT,
		syscall.SIGKILL,
		syscall.SIGPIPE,
		syscall.SIGPOLL,
		syscall.SIGPROF,
		syscall.SIGPWR,
		syscall.SIGQUIT,
		syscall.SIGSEGV,
		syscall.SIGSTKFLT,
		syscall.SIGSTOP,
		syscall.SIGSYS,
		syscall.SIGTERM,
		syscall.SIGTRAP,
		syscall.SIGTSTP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
		syscall.SIGUNUSED,
		syscall.SIGURG,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGVTALRM,
		syscall.SIGWINCH,
		syscall.SIGXCPU,
		syscall.SIGXFSZ,
	)
}
