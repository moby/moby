// +build linux

package namespaces

/*
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/limits.h>
#include <linux/sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <getopt.h>

static const kBufSize = 256;

void get_args(int *argc, char ***argv) {
	// Read argv
	int fd = open("/proc/self/cmdline", O_RDONLY);

	// Read the whole commandline.
	ssize_t contents_size = 0;
	ssize_t contents_offset = 0;
	char *contents = NULL;
	ssize_t bytes_read = 0;
	do {
		contents_size += kBufSize;
		contents = (char *) realloc(contents, contents_size);
		bytes_read = read(fd, contents + contents_offset, contents_size - contents_offset);
		contents_offset += bytes_read;
	} while (bytes_read > 0);
	close(fd);

	// Parse the commandline into an argv. /proc/self/cmdline has \0 delimited args.
	ssize_t i;
	*argc = 0;
	for (i = 0; i < contents_offset; i++) {
		if (contents[i] == '\0') {
			(*argc)++;
		}
	}
	*argv = (char **) malloc(sizeof(char *) * ((*argc) + 1));
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
int setns(int fd, int nstype) {
  return syscall(SYS_setns, fd, nstype);
}
#endif
#endif

void print_usage() {
	fprintf(stderr, "<binary> nsenter --nspid <pid> --containerjson <container_json> -- cmd1 arg1 arg2...\n");
}

void nsenter() {
	int argc;
	char **argv;
	get_args(&argc, &argv);

	// Ignore if this is not for us.
	if (argc < 2 || strcmp(argv[1], "nsenter") != 0) {
		return;
	}

	// USAGE: <binary> nsenter <PID> <process label> <container JSON> <argv>...
	if (argc < 6) {
		fprintf(stderr, "nsenter: Incorrect usage, not enough arguments\n");
		exit(1);
	}

	static const struct option longopts[] = {
		{ "nspid",         required_argument, NULL, 'n' },
		{ "containerjson", required_argument, NULL, 'c' },
		{ NULL,            0,                 NULL,  0  }
	};

	int c;
	pid_t init_pid = -1;
	char *init_pid_str = NULL;
	char *container_json = NULL;
	while ((c = getopt_long_only(argc, argv, "n:s:c:", longopts, NULL)) != -1) {
		switch (c) {
		case 'n':
			init_pid_str = optarg;
			break;
		case 'c':
			container_json = optarg;
			break;
		}
	}

	if (container_json == NULL || init_pid_str == NULL) {
		print_usage();
		exit(1);
	}

	init_pid = strtol(init_pid_str, NULL, 10);
	if (errno != 0 || init_pid <= 0) {
		fprintf(stderr, "nsenter: Failed to parse PID from \"%s\" with error: \"%s\"\n", init_pid_str, strerror(errno));
		print_usage();
		exit(1);
	}

	argc -= 3;
	argv += 3;

	// Setns on all supported namespaces.
	char ns_dir[PATH_MAX];
	memset(ns_dir, 0, PATH_MAX);
	snprintf(ns_dir, PATH_MAX - 1, "/proc/%d/ns/", init_pid);
	struct dirent *dent;
	DIR *dir = opendir(ns_dir);
	if (dir == NULL) {
		fprintf(stderr, "nsenter: Failed to open directory \"%s\" with error: \"%s\"\n", ns_dir, strerror(errno));
		exit(1);
	}

	while((dent = readdir(dir)) != NULL) {
		if(strcmp(dent->d_name, ".") == 0 || strcmp(dent->d_name, "..") == 0 || strcmp(dent->d_name, "user") == 0) {
			continue;
		}

		// Get and open the namespace for the init we are joining..
		char buf[PATH_MAX];
		memset(buf, 0, PATH_MAX);
		snprintf(buf, PATH_MAX - 1, "%s%s", ns_dir, dent->d_name);
		int fd = open(buf, O_RDONLY);
		if (fd == -1) {
			fprintf(stderr, "nsenter: Failed to open ns file \"%s\" for ns \"%s\" with error: \"%s\"\n", buf, dent->d_name, strerror(errno));
			exit(1);
		}

		// Set the namespace.
		if (setns(fd, 0) == -1) {
			fprintf(stderr, "nsenter: Failed to setns for \"%s\" with error: \"%s\"\n", dent->d_name, strerror(errno));
			exit(1);
		}
		close(fd);
	}
	closedir(dir);

	// We must fork to actually enter the PID namespace.
	int child = fork();
	if (child == 0) {
		// Finish executing, let the Go runtime take over.
		return;
	} else {
		// Parent, wait for the child.
		int status = 0;
		if (waitpid(child, &status, 0) == -1) {
			fprintf(stderr, "nsenter: Failed to waitpid with error: \"%s\"\n", strerror(errno));
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

__attribute__((constructor)) init() {
	nsenter();
}
*/
import "C"
