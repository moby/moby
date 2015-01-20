// +build cgo
//
// formated with indent -linux nsenter.c

#include <errno.h>
#include <fcntl.h>
#include <linux/limits.h>
#include <linux/sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <sys/types.h>
#include <unistd.h>
#include <getopt.h>

#define pr_perror(fmt, ...) fprintf(stderr, "nsenter: " fmt ": %m\n", ##__VA_ARGS__)

static const kBufSize = 256;
static const char *kNsEnter = "nsenter";

void get_args(int *argc, char ***argv)
{
	// Read argv
	int fd = open("/proc/self/cmdline", O_RDONLY);
	if (fd < 0) {
		pr_perror("Unable to open /proc/self/cmdline");
		exit(1);
	}
	// Read the whole commandline.
	ssize_t contents_size = 0;
	ssize_t contents_offset = 0;
	char *contents = NULL;
	ssize_t bytes_read = 0;
	do {
		contents_size += kBufSize;
		contents = (char *)realloc(contents, contents_size);
		bytes_read =
		    read(fd, contents + contents_offset,
			 contents_size - contents_offset);
		if (bytes_read < 0) {
			pr_perror("Unable to read from /proc/self/cmdline");
			exit(1);
		}
		contents_offset += bytes_read;
	}
	while (bytes_read > 0);
	close(fd);

	// Parse the commandline into an argv. /proc/self/cmdline has \0 delimited args.
	ssize_t i;
	*argc = 0;
	for (i = 0; i < contents_offset; i++) {
		if (contents[i] == '\0') {
			(*argc)++;
		}
	}
	*argv = (char **)malloc(sizeof(char *) * ((*argc) + 1));
	int idx;
	for (idx = 0; idx < (*argc); idx++) {
		(*argv)[idx] = contents;
		contents += strlen(contents) + 1;
	}
	(*argv)[*argc] = NULL;
}

// Use raw setns syscall for versions of glibc that don't include it (namely glibc-2.12)
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#define _GNU_SOURCE
#include <sched.h>
#include "syscall.h"
#ifdef SYS_setns
int setns(int fd, int nstype)
{
	return syscall(SYS_setns, fd, nstype);
}
#endif
#endif

void print_usage()
{
	fprintf(stderr,
		"nsenter --nspid <pid> --console <console> -- cmd1 arg1 arg2...\n");
}

void nsenter()
{
	int argc, c;
	char **argv;
	get_args(&argc, &argv);

	// check argv 0 to ensure that we are supposed to setns
	// we use strncmp to test for a value of "nsenter" but also allows alternate implmentations
	// after the setns code path to continue to use the argv 0 to determine actions to be run
	// resulting in the ability to specify "nsenter-mknod", "nsenter-exec", etc...
	if (strncmp(argv[0], kNsEnter, strlen(kNsEnter)) != 0) {
		return;
	}
#ifdef PR_SET_CHILD_SUBREAPER
	if (prctl(PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) == -1) {
		pr_perror("Failed to set child subreaper");
		exit(1);
	}
#endif

	static const struct option longopts[] = {
		{"nspid", required_argument, NULL, 'n'},
		{"console", required_argument, NULL, 't'},
		{NULL, 0, NULL, 0}
	};

	pid_t init_pid = -1;
	char *init_pid_str = NULL;
	char *console = NULL;
	while ((c = getopt_long_only(argc, argv, "n:c:", longopts, NULL)) != -1) {
		switch (c) {
		case 'n':
			init_pid_str = optarg;
			break;
		case 't':
			console = optarg;
			break;
		}
	}

	if (init_pid_str == NULL) {
		print_usage();
		exit(1);
	}

	init_pid = strtol(init_pid_str, NULL, 10);
	if ((init_pid == 0 && errno == EINVAL) || errno == ERANGE) {
		pr_perror("Failed to parse PID from \"%s\" with output \"%d\"",
			  init_pid_str, init_pid);
		print_usage();
		exit(1);
	}

	argc -= 3;
	argv += 3;

	if (setsid() == -1) {
		pr_perror("setsid failed");
		exit(1);
	}
	// before we setns we need to dup the console
	int consolefd = -1;
	if (console != NULL) {
		consolefd = open(console, O_RDWR);
		if (consolefd < 0) {
			pr_perror("Failed to open console %s", console);
			exit(1);
		}
	}
	// blocking until the parent placed the process inside correct cgroups.
	unsigned char s;
	if (read(3, &s, 1) != 1 || s != '1') {
		pr_perror("failed to receive synchronization data from parent");
		exit(1);
	}
	// Setns on all supported namespaces.
	char ns_dir[PATH_MAX];
	memset(ns_dir, 0, PATH_MAX);
	snprintf(ns_dir, PATH_MAX - 1, "/proc/%d/ns/", init_pid);

	int ns_dir_fd;
	ns_dir_fd = open(ns_dir, O_RDONLY | O_DIRECTORY);
	if (ns_dir_fd < 0) {
		pr_perror("Unable to open %s", ns_dir);
		exit(1);
	}

	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };
	const int num = sizeof(namespaces) / sizeof(char *);
	int i;
	for (i = 0; i < num; i++) {
		// A zombie process has links on namespaces, but they can't be opened
		struct stat st;
		if (fstatat(ns_dir_fd, namespaces[i], &st, AT_SYMLINK_NOFOLLOW)
		    == -1) {
			if (errno == ENOENT)
				continue;
			pr_perror("Failed to stat ns file %s for ns %s",
				  ns_dir, namespaces[i]);
			exit(1);
		}

		int fd = openat(ns_dir_fd, namespaces[i], O_RDONLY);
		if (fd == -1) {
			pr_perror("Failed to open ns file %s for ns %s",
				  ns_dir, namespaces[i]);
			exit(1);
		}
		// Set the namespace.
		if (setns(fd, 0) == -1) {
			pr_perror("Failed to setns for %s", namespaces[i]);
			exit(1);
		}
		close(fd);
	}
	close(ns_dir_fd);

	// We must fork to actually enter the PID namespace.
	int child = fork();
	if (child == -1) {
		pr_perror("Unable to fork a process");
		exit(1);
	}
	if (child == 0) {
		if (consolefd != -1) {
			if (dup2(consolefd, STDIN_FILENO) != 0) {
				pr_perror("Failed to dup 0");
				exit(1);
			}
			if (dup2(consolefd, STDOUT_FILENO) != STDOUT_FILENO) {
				pr_perror("Failed to dup 1");
				exit(1);
			}
			if (dup2(consolefd, STDERR_FILENO) != STDERR_FILENO) {
				pr_perror("Failed to dup 2\n");
				exit(1);
			}
		}
		// Finish executing, let the Go runtime take over.
		return;
	} else {
		// Parent, wait for the child.
		int status = 0;
		if (waitpid(child, &status, 0) == -1) {
			pr_perror("nsenter: Failed to waitpid with error");
			exit(1);
		}
		// Forward the child's exit code or re-send its death signal.
		if (WIFEXITED(status)) {
			exit(WEXITSTATUS(status));
		} else if (WIFSIGNALED(status)) {
			kill(getpid(), WTERMSIG(status));
		}

		exit(1);
	}

	return;
}
