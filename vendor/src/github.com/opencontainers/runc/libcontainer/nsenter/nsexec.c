#define _GNU_SOURCE
#include <endian.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/limits.h>
#include <sys/socket.h>
#include <linux/netlink.h>
#include <sched.h>
#include <setjmp.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/types.h>
#include <unistd.h>

#include <bits/sockaddr.h>
#include <linux/netlink.h>
#include <linux/types.h>
#include <stdint.h>
#include <sys/socket.h>

// All arguments should be above the stack because it grows down
struct clone_arg {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char     stack[4096] __attribute__((aligned(16)));
	char     stack_ptr[0];
	jmp_buf *env;
};

struct nsenter_config {
	uint32_t cloneflags;
	char     *uidmap;
	int      uidmap_len;
	char     *gidmap;
	int      gidmap_len;
	uint8_t  is_setgroup;
};

// list of known message types we want to send to bootstrap program
// These are defined in libcontainer/message_linux.go
#define INIT_MSG	    62000
#define CLONE_FLAGS_ATTR    27281
#define CONSOLE_PATH_ATTR   27282
#define NS_PATHS_ATTR	    27283
#define UIDMAP_ATTR	    27284
#define GIDMAP_ATTR	    27285
#define SETGROUP_ATTR	    27286

// Use raw setns syscall for versions of glibc that don't include it
// (namely glibc-2.12)
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
    #define _GNU_SOURCE
    #include "syscall.h"
    #if defined(__NR_setns) && !defined(SYS_setns)
	#define SYS_setns __NR_setns
    #endif

    #ifdef SYS_setns
	int setns(int fd, int nstype)
	{
	    return syscall(SYS_setns, fd, nstype);
	}
    #endif
#endif

#define pr_perror(fmt, ...)                                                    \
	fprintf(stderr, "nsenter: " fmt ": %m\n", ##__VA_ARGS__)

static int child_func(void *_arg)
{
    struct clone_arg *arg = (struct clone_arg *)_arg;
    longjmp(*arg->env, 1);
}

static int clone_parent(jmp_buf *env, int flags) __attribute__((noinline));
static int clone_parent(jmp_buf *env, int flags)
{
	struct clone_arg ca;
	int		 child;

	ca.env = env;
	child  = clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD | flags,
		      &ca);
	return child;
}

// get init pipe from the parent. It's used to read bootstrap data, and to
// write pid to after nsexec finishes setting up the environment.
static int get_init_pipe()
{
	char	buf[PATH_MAX];
	char	*initpipe;
	int	pipenum = -1;

	initpipe = getenv("_LIBCONTAINER_INITPIPE");
	if (initpipe == NULL) {
		return -1;
	}

	pipenum = atoi(initpipe);
	snprintf(buf, sizeof(buf), "%d", pipenum);
	if (strcmp(initpipe, buf)) {
		pr_perror("Unable to parse _LIBCONTAINER_INITPIPE");
		exit(1);
	}

	return pipenum;
}

// num_namespaces returns the number of additional namespaces to setns. The
// argument is a comma-separated string of namespace paths.
static int num_namespaces(char *nspaths)
{
	int i;
	int size = 0;

	for (i = 0; nspaths[i]; i++) {
		if (nspaths[i] == ',') {
			size += 1;
		}
	}

	return size + 1;
}

static uint32_t readint32(char *buf)
{
    return *(uint32_t *)buf;
}

static uint8_t readint8(char *buf)
{
    return *(uint8_t *)buf;
}

static void update_process_idmap(char *pathfmt, int pid, char *map, int map_len)
{
	char    buf[PATH_MAX];
	int	len;
	int	fd;

	len = snprintf(buf, sizeof(buf), pathfmt, pid);
	if (len < 0) {
		pr_perror("failed to construct '%s' for %d", pathfmt, pid);
		exit(1);
	}

	fd = open(buf, O_RDWR);
	if (fd == -1) {
		pr_perror("failed to open %s", buf);
		exit(1);
	}

	len = write(fd, map, map_len);
	if (len == -1) {
		pr_perror("failed to write to %s", buf);
		exit(1);
	} else if (len != map_len) {
		fprintf(stderr, "Failed to write data to %s (%d/%d)",
			buf, len, map_len);
		exit(1);
	}

	close(fd);
}

static void update_process_uidmap(int pid, char *map, int map_len)
{
	if ((map == NULL) || (map_len <= 0)) {
		return;
	}

	update_process_idmap("/proc/%d/uid_map", pid, map, map_len);
}

static void update_process_gidmap(int pid, uint8_t is_setgroup, char *map, int map_len)
{
	if ((map == NULL) || (map_len <= 0)) {
		return;
	}

	if (is_setgroup == 1) {
		int	fd;
		int	len;
		char	buf[PATH_MAX];

		len = snprintf(buf, sizeof(buf), "/proc/%d/setgroups", pid);
		if (len < 0) {
			pr_perror("failed to get setgroups path for %d", pid);
			exit(1);
		}

		fd = open(buf, O_RDWR);
		if (fd == -1) {
			pr_perror("failed to open %s", buf);
			exit(1);
		}
		if (write(fd, "allow", 5) != 5) {
			// If the kernel is too old to support
			// /proc/PID/setgroups, write will return
			// ENOENT; this is OK.
			if (errno != ENOENT) {
				pr_perror("failed to write allow to %s", buf);
				exit(1);
			}
		}
		close(fd);
	}

	update_process_idmap("/proc/%d/gid_map", pid, map, map_len);
}


static void start_child(int pipenum, jmp_buf *env, int syncpipe[2],
		 struct nsenter_config *config)
{
	int     len;
	int     childpid;
	char    buf[PATH_MAX];
	uint8_t syncbyte = 1;

	// We must fork to actually enter the PID namespace, use CLONE_PARENT
	// so the child can have the right parent, and we don't need to forward
	// the child's exit code or resend its death signal.
	childpid = clone_parent(env, config->cloneflags);
	if (childpid < 0) {
		pr_perror("Unable to fork");
		exit(1);
	}

	// update uid_map and gid_map for the child process if they
	// were provided
	update_process_uidmap(childpid, config->uidmap, config->uidmap_len);

	update_process_gidmap(childpid, config->is_setgroup, config->gidmap, config->gidmap_len);

	// Send the sync signal to the child
	close(syncpipe[0]);
	syncbyte = 1;
	if (write(syncpipe[1], &syncbyte, 1) != 1) {
		pr_perror("failed to write sync byte to child");
		exit(1);
	}

	// Send the child pid back to our parent
	len = snprintf(buf, sizeof(buf), "{ \"pid\" : %d }\n", childpid);
	if ((len < 0) || (write(pipenum, buf, len) != len)) {
		pr_perror("Unable to send a child pid");
		kill(childpid, SIGKILL);
		exit(1);
	}

	exit(0);
}

static void process_nl_attributes(int pipenum, char *data, int data_size)
{
	jmp_buf			env;
	struct nsenter_config	config	    = {0};
	struct nlattr		*nlattr;
	int			payload_len;
	int			start       = 0;
	int			consolefd   = -1;
	int			syncpipe[2] = {-1, -1};

	while (start < data_size) {
		nlattr = (struct nlattr *)(data + start);
		start += NLA_HDRLEN;
		payload_len = nlattr->nla_len - NLA_HDRLEN;

		if (nlattr->nla_type == CLONE_FLAGS_ATTR) {
			config.cloneflags = readint32(data + start);
		} else if (nlattr->nla_type == CONSOLE_PATH_ATTR) {
			// get the console path before setns because it may
			// change mnt namespace
			consolefd = open(data + start, O_RDWR);
			if (consolefd < 0) {
				pr_perror("Failed to open console %s",
					  data + start);
				exit(1);
			}
		} else if (nlattr->nla_type == NS_PATHS_ATTR) {
			// if custom namespaces are required, open all
			// descriptors and perform setns on them
			int	i;
			int	nslen = num_namespaces(data + start);
			int	fds[nslen];
			char	*nslist[nslen];
			char	*ns;
			char	*saveptr;

			for (i = 0; i < nslen; i++) {
				char *str = NULL;

				if (i == 0) {
					str = data + start;
				}
				ns = strtok_r(str, ",", &saveptr);
				if (ns == NULL) {
					break;
				}
				fds[i] = open(ns, O_RDONLY);
				if (fds[i] == -1) {
					pr_perror("Failed to open %s", ns);
					exit(1);
				}
				nslist[i] = ns;
			}

			for (i = 0; i < nslen; i++) {
				if (setns(fds[i], 0) != 0) {
					pr_perror("Failed to setns to %s", nslist[i]);
					exit(1);
				}
				close(fds[i]);
			}
		} else if (nlattr->nla_type == UIDMAP_ATTR) {
			config.uidmap     = data + start;
			config.uidmap_len = payload_len;
		} else if (nlattr->nla_type == GIDMAP_ATTR) {
			config.gidmap     = data + start;
			config.gidmap_len = payload_len;
		} else if (nlattr->nla_type == SETGROUP_ATTR) {
			config.is_setgroup = readint8(data + start);
		} else {
			pr_perror("Unknown netlink message type %d",
				  nlattr->nla_type);
			exit(1);
		}

		start += NLA_ALIGN(payload_len);
	}

	// required clone_flags to be passed
	if (config.cloneflags == -1) {
		pr_perror("Missing clone_flags");
		exit(1);
	}
	// prepare sync pipe between parent and child. We need this to let the
	// child
	// know that the parent has finished setting up
	if (pipe(syncpipe) != 0) {
		pr_perror("Failed to setup sync pipe between parent and child");
		exit(1);
	}

	if (setjmp(env) == 1) {
		// Child
		uint8_t s = 0;

		// close the writing side of pipe
		close(syncpipe[1]);

		// sync with parent
		if ((read(syncpipe[0], &s, 1) != 1) || (s != 1)) {
			pr_perror("Failed to read sync byte from parent");
			exit(1);
		}

		if (setsid() == -1) {
			pr_perror("setsid failed");
			exit(1);
		}

		if (setuid(0) == -1) {
			pr_perror("setuid failed");
			exit(1);
		}

		if (setgid(0) == -1) {
			pr_perror("setgid failed");
			exit(1);
		}

		if (consolefd != -1) {
			if (ioctl(consolefd, TIOCSCTTY, 0) == -1) {
				pr_perror("ioctl TIOCSCTTY failed");
				exit(1);
			}
			if (dup3(consolefd, STDIN_FILENO, 0) != STDIN_FILENO) {
				pr_perror("Failed to dup stdin");
				exit(1);
			}
			if (dup3(consolefd, STDOUT_FILENO, 0) != STDOUT_FILENO) {
				pr_perror("Failed to dup stdout");
				exit(1);
			}
			if (dup3(consolefd, STDERR_FILENO, 0) != STDERR_FILENO) {
				pr_perror("Failed to dup stderr");
				exit(1);
			}
		}

		// Finish executing, let the Go runtime take over.
		return;
	}

	// Parent
	start_child(pipenum, &env, syncpipe, &config);
}

void nsexec(void)
{
	int pipenum;

	// if we dont have init pipe, then just return to the parent
	pipenum = get_init_pipe();
	if (pipenum == -1) {
		return;
	}

	// Retrieve the netlink header
	struct nlmsghdr nl_msg_hdr;
	int		len;

	if ((len = read(pipenum, &nl_msg_hdr, NLMSG_HDRLEN)) != NLMSG_HDRLEN) {
		pr_perror("Invalid netlink header length %d", len);
		exit(1);
	}

	if (nl_msg_hdr.nlmsg_type == NLMSG_ERROR) {
		pr_perror("Failed to read netlink message");
		exit(1);
	}

	if (nl_msg_hdr.nlmsg_type != INIT_MSG) {
		pr_perror("Unexpected msg type %d", nl_msg_hdr.nlmsg_type);
		exit(1);
	}

	// Retrieve data
	int  nl_total_size = NLMSG_PAYLOAD(&nl_msg_hdr, 0);
	char data[nl_total_size];

	if ((len = read(pipenum, data, nl_total_size)) != nl_total_size) {
		pr_perror("Failed to read netlink payload, %d != %d", len,
			  nl_total_size);
		exit(1);
	}

	process_nl_attributes(pipenum, data, nl_total_size);
}
