/*-
 * Copyright (c) 2011 NetApp, Inc.
 * Copyright (c) 2015 xhyve developers
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY NETAPP, INC ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL NETAPP, INC OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 *
 * $FreeBSD$
 */

#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <sys/uio.h>

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <xhyve/support/misc.h>
#include <xhyve/inout.h>
#include <xhyve/dbgport.h>
#include <xhyve/pci_lpc.h>

#define	BVM_DBG_PORT 0x224
#define	BVM_DBG_SIG ('B' << 8 | 'V')

static int listen_fd, conn_fd;

static struct sockaddr_in saddrin;

static int
dbg_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes, uint32_t *eax,
	UNUSED void *arg)
{
	char ch;
	int nwritten, nread, printonce;

	if (bytes == 2 && in) {
		*eax = BVM_DBG_SIG;
		return (0);
	}

	if (bytes != 4)
		return (-1);

again:
	printonce = 0;
	while (conn_fd < 0) {
		if (!printonce) {
			printf("Waiting for connection from gdb\r\n");
			printonce = 1;
		}
		conn_fd = accept(listen_fd, NULL, NULL);
		if (conn_fd >= 0)
			fcntl(conn_fd, F_SETFL, O_NONBLOCK);
		else if (errno != EINTR)
			perror("accept");
	}

	if (in) {
		nread = (int) read(conn_fd, &ch, 1);
		if (nread == -1 && errno == EAGAIN)
			*eax = (uint32_t) (-1);
		else if (nread == 1)
			*eax = (uint32_t) ch;
		else {
			close(conn_fd);
			conn_fd = -1;
			goto again;
		}
	} else {
		ch = (char) *eax;
		nwritten = (int) write(conn_fd, &ch, 1);
		if (nwritten != 1) {
			close(conn_fd);
			conn_fd = -1;
			goto again;
		}
	}
	return (0);
}

static struct inout_port dbgport = {
	"bvmdbg",
	BVM_DBG_PORT,
	1,
	IOPORT_F_INOUT,
	dbg_handler,
	NULL
};

SYSRES_IO(BVM_DBG_PORT, 4);

void
init_dbgport(int sport)
{
	conn_fd = -1;

	if ((listen_fd = socket(AF_INET, SOCK_STREAM, 0)) < 0) {
		perror("socket");
		exit(1);
	}

	saddrin.sin_len = sizeof(saddrin);
	saddrin.sin_family = AF_INET;
	saddrin.sin_addr.s_addr = htonl(INADDR_ANY);
	saddrin.sin_port = htons(sport);

	if (bind(listen_fd, (struct sockaddr *)&saddrin, sizeof(saddrin)) < 0) {
		perror("bind");
		exit(1);
	}

	if (listen(listen_fd, 1) < 0) {
		perror("listen");
		exit(1);
	}

	register_inout(&dbgport);
}
