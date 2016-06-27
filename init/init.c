#define _GNU_SOURCE

#include <errno.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/signalfd.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

static void handle_error()
{
	fprintf(stderr, "error: %s\n", strerror(errno));
	exit(EXIT_FAILURE);
}

// exit_status returns the correct exist status for a process
// depending if it existed or as signaled
static int exit_status(int status)
{
	if (WIFSIGNALED(status)) {
		return WTERMSIG(status) + 128;
	}
	return WEXITSTATUS(status);
}

typedef struct process_t {
	char **args;
	sigset_t set;
	pid_t pid;
} process_t;

typedef struct reaper_t {
	int fd;
	sigset_t parent_set;
	sigset_t child_set;
	process_t *child;
} reaper_t;

// reaper_init initializes the reaper with the provided process.
// it also sets up the signal handlers and child handlers for restore 
// when the child is execed
int reaper_init(reaper_t * reaper, process_t * process)
{
	int i;
	int sync_signals[] = {
		SIGABRT,
		SIGBUS,
		SIGFPE,
		SIGILL,
		SIGSEGV,
		SIGSYS,
		SIGTRAP,
	};

	if (sigfillset(&reaper->parent_set) != 0) {
		return -1;
	}
	for (i = 0; i < (sizeof(sync_signals) / sizeof(int)); i++) {
		if (sigdelset(&reaper->parent_set, sync_signals[i]) != 0) {
			return -1;
		}
	}
	if (sigprocmask(SIG_SETMASK, &reaper->parent_set, &reaper->child_set) !=
	    0) {
		return -1;
	}
	reaper->fd = signalfd(-1, &reaper->parent_set, SFD_CLOEXEC);
	if (reaper->fd == -1) {
		return -1;
	}
	reaper->child = process;
	process->set = reaper->child_set;
	return 0;
}

// reaper_reap reaps any dead processes.  If the process that is reaped 
// is the child process that we spawned get its exit status and exit this program
int reaper_reap(reaper_t * reaper)
{
	int status;
	struct rusage usage;
	for (;;) {
		pid_t pid = wait4(-1, &status, WNOHANG, &usage);
		if (pid < 0) {
			if (errno == ECHILD) {
				return 0;
			}
			return pid;
		}
		if (reaper->child->pid == pid) {
			close(reaper->fd);
			exit(exit_status(status));
		}
	}
	return 0;
}

// process_signal sends the specified signal to the process
int process_signal(process_t * process, int signal)
{
	return kill(process->pid, signal);
}

// process_exec executes the new child process under supervision of the init
int process_exec(process_t * process)
{
	pid_t pid = fork();
	if (pid < 0) {
		return pid;
	}
	if (pid == 0) {
		if (sigprocmask(SIG_SETMASK, &process->set, NULL) != 0) {
			handle_error();
		}
		execvp(process->args[0], process->args);
		handle_error();
	}
	process->pid = pid;
	return 0;
}

int main(int argc, char **argv)
{
	process_t process;
	reaper_t reaper;

	if (reaper_init(&reaper, &process) != 0) {
		handle_error();
	}
	// setup the process
	process.args = &argv[1];

	if (process_exec(&process) != 0) {
		handle_error();
	}

	for (;;) {
		struct signalfd_siginfo info;
		int i = read(reaper.fd, &info, sizeof(info));
		if (i != sizeof(info)) {
			fprintf(stderr, "read: invalid size of siginfo\n");
			exit(EXIT_FAILURE);
		}
		uint32_t sig = info.ssi_signo;
		switch (sig) {
		case SIGCHLD:
			reaper_reap(&reaper);
			break;
		default:
			process_signal(&process, sig);
			break;
		}
	}
}
