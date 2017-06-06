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

/*
 *
 * The vmnet support is ported from the MirageOS project:
 *
 * https://github.com/mirage/ocaml-vmnet
 *
 *      Copyright (C) 2014 Anil Madhavapeddy <anil@recoil.org>
 *
 *      Permission to use, copy, modify, and distribute this software for any
 *      purpose with or without fee is hereby granted, provided that the above
 *      copyright notice and this permission notice appear in all copies.
 *
 *      THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 *      WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 *      MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 *      ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 *      WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 *      ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 *      OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <strings.h>
#include <pthread.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <assert.h>
#include <ctype.h>
#include <sys/select.h>
#include <sys/param.h>
#include <sys/uio.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/time.h>
#include <net/ethernet.h>
#include <uuid/uuid.h>
#include <xhyve/support/misc.h>
#include <xhyve/support/atomic.h>
#include <xhyve/support/linker_set.h>
#include <xhyve/support/md5.h>
#include <xhyve/support/uuid.h>
#include <xhyve/xhyve.h>
#include <xhyve/pci_emul.h>
#include <xhyve/mevent.h>
#include <xhyve/virtio.h>

#define WPRINTF(format, ...) printf(format, __VA_ARGS__)

#define VTNET_RINGSZ 1024
#define VTNET_MAXSEGS 32

/*
 * wire protocol
 */
struct msg_init {
	uint8_t magic[5]; /* VMN3T */
	uint32_t version;
	uint8_t commit[40];
} __packed;

#define CMD_ETHERNET 1
struct msg_common {
	uint8_t command;
} __packed;

struct msg_ethernet {
	uint8_t command; /* CMD_ETHERNET */
	char uuid[36];
} __packed;

struct vif_info {
	uint16_t mtu;
	uint16_t max_packet_size;
	uint8_t mac[6];
} __packed;


/*
 * Host capabilities.  Note that we only offer a few of these.
 */
#define	VIRTIO_NET_F_CSUM	(1 <<  0) /* host handles partial cksum */
#define	VIRTIO_NET_F_GUEST_CSUM	(1 <<  1) /* guest handles partial cksum */
#define	VIRTIO_NET_F_MAC	(1 <<  5) /* host supplies MAC */
#define	VIRTIO_NET_F_GSO_DEPREC	(1 <<  6) /* deprecated: host handles GSO */
#define	VIRTIO_NET_F_GUEST_TSO4	(1 <<  7) /* guest can rcv TSOv4 */
#define	VIRTIO_NET_F_GUEST_TSO6	(1 <<  8) /* guest can rcv TSOv6 */
#define	VIRTIO_NET_F_GUEST_ECN	(1 <<  9) /* guest can rcv TSO with ECN */
#define	VIRTIO_NET_F_GUEST_UFO	(1 << 10) /* guest can rcv UFO */
#define	VIRTIO_NET_F_HOST_TSO4	(1 << 11) /* host can rcv TSOv4 */
#define	VIRTIO_NET_F_HOST_TSO6	(1 << 12) /* host can rcv TSOv6 */
#define	VIRTIO_NET_F_HOST_ECN	(1 << 13) /* host can rcv TSO with ECN */
#define	VIRTIO_NET_F_HOST_UFO	(1 << 14) /* host can rcv UFO */
#define	VIRTIO_NET_F_MRG_RXBUF	(1 << 15) /* host can merge RX buffers */
#define	VIRTIO_NET_F_STATUS	(1 << 16) /* config status field available */
#define	VIRTIO_NET_F_CTRL_VQ	(1 << 17) /* control channel available */
#define	VIRTIO_NET_F_CTRL_RX	(1 << 18) /* control channel RX mode support */
#define	VIRTIO_NET_F_CTRL_VLAN	(1 << 19) /* control channel VLAN filtering */
#define	VIRTIO_NET_F_GUEST_ANNOUNCE \
				(1 << 21) /* guest can send gratuitous pkts */

#define VTNET_S_HOSTCAPS \
	(VIRTIO_NET_F_MAC | VIRTIO_NET_F_MRG_RXBUF | VIRTIO_NET_F_STATUS | \
	VIRTIO_F_NOTIFY_ON_EMPTY)

/*
 * PCI config-space "registers"
 */
struct virtio_net_config {
	uint8_t mac[6];
	uint16_t status;
} __packed;

/*
 * Queue definitions.
 */
#define VTNET_RXQ	0
#define VTNET_TXQ	1
#define VTNET_CTLQ	2	/* NB: not yet supported */

#define VTNET_MAXQ	3

/*
 * Fixed network header size
 */
struct virtio_net_rxhdr {
	uint8_t		vrh_flags;
	uint8_t		vrh_gso_type;
	uint16_t	vrh_hdr_len;
	uint16_t	vrh_gso_size;
	uint16_t	vrh_csum_start;
	uint16_t	vrh_csum_offset;
	uint16_t	vrh_bufs;
} __packed;

/*
 * Debug printf
 */
static int pci_vtnet_debug;
#define DPRINTF(params) if (pci_vtnet_debug) printf params

/*
 * Per-device softc
 */
struct pci_vtnet_softc {
	struct virtio_softc vsc_vs;
	struct vqueue_info vsc_queues[VTNET_MAXQ - 1];
	pthread_mutex_t vsc_mtx;
	struct vpnkit_state *state;

	int		vsc_rx_ready;
	volatile int	resetting;	/* set and checked outside lock */

	uint64_t	vsc_features;	/* negotiated features */

	struct virtio_net_config vsc_config;

	pthread_mutex_t	rx_mtx;
	int		rx_in_progress;
	int		rx_vhdrlen;
	int		rx_merge;	/* merged rx bufs in use */

	pthread_t 	tx_tid;
	pthread_mutex_t	tx_mtx;
	pthread_cond_t	tx_cond;
	int		tx_in_progress;
};

static void pci_vtnet_reset(void *);
/* static void pci_vtnet_notify(void *, struct vqueue_info *); */
static int pci_vtnet_cfgread(void *, int, int, uint32_t *);
static int pci_vtnet_cfgwrite(void *, int, int, uint32_t);
static void pci_vtnet_neg_features(void *, uint64_t);

static struct virtio_consts vtnet_vi_consts = {
	"vpnkit",		/* our name */
	VTNET_MAXQ - 1,		/* we currently support 2 virtqueues */
	sizeof(struct virtio_net_config), /* config reg size */
	pci_vtnet_reset,	/* reset */
	NULL,			/* device-wide qnotify -- not used */
	pci_vtnet_cfgread,	/* read PCI config */
	pci_vtnet_cfgwrite,	/* write PCI config */
	pci_vtnet_neg_features,	/* apply negotiated features */
	VTNET_S_HOSTCAPS,	/* our capabilities */
};

struct vpnkit_state {
	int fd;
	struct vif_info vif;
};

static int really_read(int fd, uint8_t *buffer, size_t total)
{
	size_t remaining = total;
	ssize_t n;

	while (remaining > 0) {
		n = read(fd, buffer, remaining);

		if (n == 0) {
			fprintf(stderr, "virtio-net-vpnkit: read EOF\n");
			goto err;
		}

		if (n < 0) {
			fprintf(stderr, "virtio-net-vpnkit: read error: %s\n",
				strerror(errno));
			goto err;
		}

		remaining -= (size_t)n;
		buffer = buffer + n;
	}

	return 0;

err:
	/* On error: stop reading from the socket and trigger a clean shutdown */
	shutdown(fd, SHUT_RD);
	return -1;
}

static int really_write(int fd, uint8_t *buffer, size_t total) {
	size_t remaining = total;
	ssize_t n;

	while (remaining > 0) {
		n = write(fd, buffer, remaining);

		if (n == 0) {
			fprintf(stderr, "virtio-net-vpnkit: wrote 0 bytes\n");
			goto err;
		}

		if (n < 0) {
			fprintf(stderr, "virtio-net-vpnkit: write error: %s\n",
				strerror(errno));
			goto err;
		}

		remaining -= (size_t) n;
		buffer = buffer + n;
	}

	return 0;

err:
	/* On error: stop listening to the socket */
	shutdown(fd, SHUT_WR);
	return -1;
}

/*
 * wire protocol
 */
static int vpnkit_connect(int fd, const char uuid[36], struct vif_info *vif)
{
	struct msg_init init_msg = {
		.magic = { 'V', 'M', 'N', '3', 'T' },
		.version = 1U,
	};

	/* msg.commit is not NULL terminated */
	assert(sizeof(VERSION_SHA1) == sizeof(init_msg.commit) + 1);
	memcpy(&init_msg.commit, VERSION_SHA1, sizeof(init_msg.commit));

	if (really_write(fd, (uint8_t *)&init_msg, sizeof(init_msg)) < 0) {
		fprintf(stderr, "virtio-net-vpnkit: failed to write init msg\n");
		return -1;
	}

	struct msg_init init_reply;
	if (really_read(fd, (uint8_t *)&init_reply, sizeof(init_reply)) < 0) {
		fprintf(stderr, "virtio-net-vpnkit: failed to read init reply\n");
		return -1;
	}

	if (memcmp(init_msg.magic, init_reply.magic, sizeof(init_reply.magic))) {
		fprintf(stderr, "virtio-net-vpnkit: bad init magic: %c%c%c%c%c\n",
			init_reply.magic[0],
			init_reply.magic[1],
			init_reply.magic[2],
			init_reply.magic[3],
			init_reply.magic[4]);
		return -1;
	}
	if (init_reply.version != 1) {
		fprintf(stderr, "virtio-net-vpnkit: bad init version %d\n",
			init_reply.version);
		return -1;
	}

	fprintf(stderr, "virtio-net-vpnkit: magic=%c%c%c%c%c version=%d commit=%*s\n",
		init_reply.magic[0], init_reply.magic[1],
		init_reply.magic[2], init_reply.magic[3],
		init_reply.magic[4],
		init_reply.version, (int)sizeof(init_reply.commit), init_reply.commit);

	struct msg_ethernet cmd_ethernet = {
		.command = CMD_ETHERNET,
	};
	memcpy(cmd_ethernet.uuid, uuid, sizeof(cmd_ethernet.uuid));

	if (really_write(fd, (uint8_t*)&cmd_ethernet, sizeof(cmd_ethernet)) < 0) {
		fprintf(stderr, "virtio-net-vpnkit: failed to write ethernet cmd\n");
		return -1;
	}

	if (really_read(fd, (uint8_t*)vif, sizeof(*vif)) < 0) {
		fprintf(stderr, "virtio-net-vpnkit: failed to read vif info\n");
		return -1;
	}

	return 0;
}

static char *
copy_up_to_comma(const char *from)
{
	char *comma = strchr(from, ',');
	char *tmp = NULL;
	if (comma == NULL) {
		tmp = strdup(from); /* rest of string */
	} else {
		size_t length = (size_t)(comma - from);
		tmp = strndup(from, length);
	}
	return tmp;
}

static int
vpnkit_create(struct pci_vtnet_softc *sc, const char *opts)
{
	const char *path = "/var/tmp/com.docker.slirp.socket";
	char *macfile = NULL;
	char *tmp = NULL;
	uuid_t uuid;
	char uuid_string[37];
	struct sockaddr_un addr;
	int fd;
	struct vpnkit_state *state = malloc(sizeof(struct vpnkit_state));
	if (!state) abort();
	bzero(state, sizeof(struct vpnkit_state));
	fprintf(stdout, "virtio-net-vpnkit: initialising, opts=\"%s\"\n",
		opts ? opts : "");

	/* Use a random uuid by default */
	uuid_generate_random(uuid);
	uuid_unparse(uuid, uuid_string);

	while (1) {
		char *next;
		if (! opts)
			break;
		next = strchr(opts, ',');
		if (next)
			next[0] = '\0';
		if (strncmp(opts, "path=", 5) == 0) {
			path = copy_up_to_comma(opts + 5);
			/* Let path leak */
		} else if (strncmp(opts, "uuid=", 5) == 0) {
			tmp = copy_up_to_comma(opts + 5);
			if (strlen(tmp) < 36) {
				fprintf(stderr, "uuids need to be 36 characters long\n");
				return 1;
			}
			memcpy(&uuid_string[0], &tmp[0], 36);
			fprintf(stdout, "Interface will have uuid %s\n", tmp);
			free(tmp);
			tmp = NULL;
		} else if (strncmp(opts, "macfile=", 8) == 0) {
			macfile = copy_up_to_comma(opts + 8);
		} else {
			fprintf(stderr, "invalid option: %s\r\n", opts);
			return 1;
		}
		if (! next)
			break;
		opts = &next[1];
	}

	state->vif.max_packet_size = 1500;
	sc->state = state;

	memset(&addr, 0, sizeof(addr));
	addr.sun_family = AF_UNIX;
	strncpy(addr.sun_path, path, sizeof(addr.sun_path)-1);
	int log_every_n_tries = 0; /* log first failure */
	do {
		if ((fd = socket(AF_UNIX, SOCK_STREAM, 0)) == -1){
			WPRINTF("Failed to create socket: %s\n", strerror(errno));
			abort();
		}
		if(connect(fd, (struct sockaddr *) &addr, sizeof(struct sockaddr_un)) != 0) {
			goto err;
		}

		if (vpnkit_connect(fd, uuid_string, &state->vif) == 0)
			/* success */
			break;

err:
		if (log_every_n_tries == 0) {
			DPRINTF(("virtio-net-vpnkit: failed to connect to %s: %m\n", path));
		}
		close(fd);
		fd = -1;
		log_every_n_tries --;
		if (log_every_n_tries < 0) log_every_n_tries = 100; /* at 100ms interval = 10s */
		usleep(100000); /* 100 ms */
	} while (fd == -1);

	state->fd = fd;

	struct vif_info *info = &state->vif;
	fprintf(stdout, "Connection established with MAC=%02x:%02x:%02x:%02x:%02x:%02x and MTU %d\n",
	  info->mac[0], info->mac[1], info->mac[2], info->mac[3], info->mac[4], info->mac[5],
		(int)info->mtu);

	if (macfile) {
		tmp = malloc(PATH_MAX);
		if (!tmp) abort();
		snprintf(tmp, PATH_MAX, "%s.tmp", macfile);
		FILE *f = fopen(tmp, "w");
		if (f == NULL) {
			DPRINTF(("Failed to write MAC to file %s: %m\n", tmp));
			return 1;
		}
		if (fprintf(f, "%02x:%02x:%02x:%02x:%02x:%02x", info->mac[0], info->mac[1],
			info->mac[2], info->mac[3], info->mac[4], info->mac[5]) < 0) {
				DPRINTF(("Failed to write MAC to file %s: %m\n", tmp));
			return 1;
		}
		fclose(f);
		if (rename(tmp, macfile) == -1) {
			DPRINTF(("Failed to write MAC to file %s: %m\n", macfile));
			return 1;
		}
	}
	return 0;
}

typedef struct pcap_hdr_s {
	uint32_t magic_number;   /* magic number */
	uint16_t version_major;  /* major version number */
	uint16_t version_minor;  /* minor version number */
	int32_t  thiszone;       /* GMT to local correction */
	uint32_t sigfigs;        /* accuracy of timestamps */
	uint32_t snaplen;        /* max length of captured packets, in octets */
	uint32_t network;        /* data link type */
} pcap_hdr_t;

typedef struct pcaprec_hdr_s {
	uint32_t ts_sec;         /* timestamp seconds */
	uint32_t ts_usec;        /* timestamp microseconds */
	uint32_t incl_len;       /* number of octets of packet saved in file */
	uint32_t orig_len;       /* actual length of packet */
} pcaprec_hdr_t;

#if 0
static void capture(unsigned char *buffer, int len){
	static int pcap_fd = 0;
	if (pcap_fd == 0){
		if ((pcap_fd = open("capture.pcap", O_WRONLY | O_CREAT | O_TRUNC)) == -1){
			fprintf(stderr, "Failed to open capture.pcap: %s\r\n", strerror(errno));
			exit(1);
		}
		struct pcap_hdr_s h;
		h.magic_number =  0xa1b2c3d4;
		h.version_major = 2;
		h.version_minor = 4;
		h.thiszone = 0;
		h.sigfigs = 0;
		h.snaplen = 2048;
		h.network = 1; /* ETHERNET */
		really_write(pcap_fd, (uint8_t*)&h, sizeof(struct pcap_hdr_s));
	};
	struct pcaprec_hdr_s h;
	struct timeval tv;
	gettimeofday(&tv, NULL);
	h.ts_sec = tv.tv_sec;
	h.ts_usec = tv.tv_usec;
	h.incl_len = len;
	h.orig_len = len;
	really_write(pcap_fd, (uint8_t*)&h, sizeof(struct pcaprec_hdr_s));
	really_write(pcap_fd, buffer, len);
}
#endif

static void hexdump(unsigned char *buffer, size_t len){
	char ascii[17];
	size_t i = 0;
	ascii[16] = '\000';
	while (i < len) {
		unsigned char c = *(buffer + i);
		printf("%02x ", c);
		ascii[i++ % 16] = isprint(c)?(signed char)c:'.';
		if ((i % 2) == 0) printf(" ");
		if ((i % 16) == 0) printf(" %s\r\n", ascii);
	};
	printf("\r\n");
}

static ssize_t
vmn_read(struct vpnkit_state *state, struct iovec *iov, int n) {
	uint8_t header[2];
	int length, remaining, i = 0;

	if (really_read(state->fd, &header[0], 2) == -1){
		DPRINTF(("virtio-net-vpnkit: read failed, pushing ACPI power button\n"));
		push_power_button();
		/* Block reading forever until we shutdown */
		for (;;){
			usleep(1000000);
		}
	}
	remaining = length = (header[0] & 0xff) | ((header[1] & 0xff) << 8);

	while (remaining > 0){
		size_t batch = min((unsigned long)remaining, iov[i].iov_len);
		if (really_read(state->fd, iov[i].iov_base, batch) == -1){
			DPRINTF(("virtio-net-vpnkit: read failed, pushing ACPI power button\n"));
			push_power_button();
			/* Block reading forever until we shutdown */
			for (;;){
				usleep(1000000);
			}
		}
		remaining -= batch;
		i++;
		assert(remaining == 0 || i < n);
	}
	DPRINTF(("Received packet of %d bytes\r\n", length));
	if (pci_vtnet_debug) {
		hexdump(iov[0].iov_base, min(iov[0].iov_len, 32));
#if 0
		capture(iov[0].iov_base, length);
#endif
	}
	return length;
}

static void
vmn_write(struct vpnkit_state *state, struct iovec *iov, int n) {
	uint8_t header[2];
	size_t length = 0;

	for (int i = 0; i < n; i++) {
		length += iov[i].iov_len;
	}

	assert(length<= state->vif.max_packet_size);

	DPRINTF(("Transmitting packet of length %zd\r\n", length));
	if (pci_vtnet_debug) {
	  hexdump(iov[0].iov_base, min(iov[0].iov_len, 32));
#if 0
	  capture(iov[0].iov_base, iov[0].iov_len);
#endif
	}
	header[0] = (length >> 0) & 0xff;
	header[1] = (length >> 8) & 0xff;
	if (really_write(state->fd, &header[0], 2) == -1){
		DPRINTF(("virtio-net-vpnkit: write failed, pushing ACPI power button"));
		push_power_button();
		return;
	}

	(void) writev(state->fd, iov, n);
}

/*
 * If the transmit thread is active then stall until it is done.
 */
static void
pci_vtnet_txwait(struct pci_vtnet_softc *sc)
{

	pthread_mutex_lock(&sc->tx_mtx);
	while (sc->tx_in_progress) {
		pthread_mutex_unlock(&sc->tx_mtx);
		usleep(10000);
		pthread_mutex_lock(&sc->tx_mtx);
	}
	pthread_mutex_unlock(&sc->tx_mtx);
}

/*
 * If the receive thread is active then stall until it is done.
 */
static void
pci_vtnet_rxwait(struct pci_vtnet_softc *sc)
{

	pthread_mutex_lock(&sc->rx_mtx);
	while (sc->rx_in_progress) {
		pthread_mutex_unlock(&sc->rx_mtx);
		usleep(10000);
		pthread_mutex_lock(&sc->rx_mtx);
	}
	pthread_mutex_unlock(&sc->rx_mtx);
}

static void
pci_vtnet_reset(void *vsc)
{
	struct pci_vtnet_softc *sc = vsc;

	DPRINTF(("vtnet: device reset requested !\n"));

	sc->resetting = 1;

	/*
	 * Wait for the transmit and receive threads to finish their
	 * processing.
	 */
	pci_vtnet_txwait(sc);
	pci_vtnet_rxwait(sc);

	sc->vsc_rx_ready = 0;
	sc->rx_merge = 1;
	sc->rx_vhdrlen = sizeof(struct virtio_net_rxhdr);

	/* now reset rings, MSI-X vectors, and negotiated capabilities */
	vi_reset_dev(&sc->vsc_vs);

	sc->resetting = 0;
}

/*
 * Called to send a buffer chain out to the tap device
 */
static void
pci_vtnet_tap_tx(struct pci_vtnet_softc *sc, struct iovec *iov, int iovcnt,
		 int len)
{
	static char pad[60]; /* all zero bytes */

	if (!sc->state)
		return;

	/*
	 * If the length is < 60, pad out to that and add the
	 * extra zero'd segment to the iov. It is guaranteed that
	 * there is always an extra iov available by the caller.
	 */
	if (len < 60) {
		iov[iovcnt].iov_base = pad;
		iov[iovcnt].iov_len = (size_t)(60 - len);
		iovcnt++;
	}
	vmn_write(sc->state, iov, iovcnt);
}

/*
 *  Called when there is read activity on the tap file descriptor.
 * Each buffer posted by the guest is assumed to be able to contain
 * an entire ethernet frame + rx header.
 *  MP note: the dummybuf is only used for discarding frames, so there
 * is no need for it to be per-vtnet or locked.
 */
static uint8_t dummybuf[2048];

static __inline struct iovec *
rx_iov_trim(struct iovec *iov, int *niov, int tlen)
{
	struct iovec *riov;

	/* XXX short-cut: assume first segment is >= tlen */
	assert(iov[0].iov_len >= (size_t)tlen);

	iov[0].iov_len -= (size_t)tlen;
	if (iov[0].iov_len == 0) {
		assert(*niov > 1);
		*niov -= 1;
		riov = &iov[1];
	} else {
		iov[0].iov_base = (void *)((uintptr_t)iov[0].iov_base +
			(size_t)tlen);
		riov = &iov[0];
	}

	return (riov);
}

static void
pci_vtnet_tap_rx(struct pci_vtnet_softc *sc)
{
	struct iovec iov[VTNET_MAXSEGS], *riov;
	struct vqueue_info *vq;
	void *vrx;
	int len, n;
	uint16_t idx;

	/*
	 * Should never be called without a valid tap fd
	 */
	assert(sc->state);

	/*
	 * But, will be called when the rx ring hasn't yet
	 * been set up or the guest is resetting the device.
	 */
	if (!sc->vsc_rx_ready || sc->resetting) {
		/*
		 * Drop the packet and try later.
		 */
		iov[0].iov_base = dummybuf;
		iov[0].iov_len = sizeof(dummybuf);
		(void) vmn_read(sc->state, iov, 1);
		return;
	}

	/*
	 * Check for available rx buffers
	 */
	vq = &sc->vsc_queues[VTNET_RXQ];
	if (!vq_has_descs(vq)) {
		/*
		 * Drop the packet and try later.  Interrupt on
		 * empty, if that's negotiated.
		 */
		iov[0].iov_base = dummybuf;
		iov[0].iov_len = sizeof(dummybuf);
		(void) vmn_read(sc->state, iov, 1);
		vq_endchains(vq, 1);
		return;
	}

	do {
		/*
		 * Get descriptor chain.
		 */
		n = vq_getchain(vq, &idx, iov, VTNET_MAXSEGS, NULL);
		assert(n >= 1 && n <= VTNET_MAXSEGS);

		/*
		 * Get a pointer to the rx header, and use the
		 * data immediately following it for the packet buffer.
		 */
		vrx = iov[0].iov_base;
		riov = rx_iov_trim(iov, &n, sc->rx_vhdrlen);

		len = (int) vmn_read(sc->state, riov, n);

		if (len < 0) {
			/*
			 * No more packets, but still some avail ring
			 * entries.  Interrupt if needed/appropriate.
			 */
			vq_retchain(vq);
			vq_endchains(vq, 0);
			return;
		}

		/*
		 * The only valid field in the rx packet header is the
		 * number of buffers if merged rx bufs were negotiated.
		 */
		memset(vrx, 0, sc->rx_vhdrlen);

		if (sc->rx_merge) {
			struct virtio_net_rxhdr *vrxh;

			vrxh = vrx;
			vrxh->vrh_bufs = 1;
		}

		/*
		 * Release this chain and handle more chains.
		 */
		vq_relchain(vq, idx, ((uint32_t) (len + sc->rx_vhdrlen)));
	} while /* (vq_has_descs(vq))*/ (0);
	/* NB: socket is in blocking mode, so rely on getting back here through
	   select() rather than readv() failing with EWOULDBLOCK */

	/* Interrupt if needed, including for NOTIFY_ON_EMPTY. */
	vq_endchains(vq, 1);
}

static void *
pci_vtnet_tap_select_func(void *vsc) {
	struct pci_vtnet_softc *sc;
	fd_set rfd;

	pthread_setname_np("net:ipc:rx");

	sc = vsc;

	assert(sc);
	assert(sc->state->fd != -1);

	FD_ZERO(&rfd);
	FD_SET(sc->state->fd, &rfd);

	while (1) {
		if (select((sc->state->fd + 1), &rfd, NULL, NULL, NULL) == -1) {
			abort();
		}

		pthread_mutex_lock(&sc->rx_mtx);
		sc->rx_in_progress = 1;
		pci_vtnet_tap_rx(sc);
		sc->rx_in_progress = 0;
		pthread_mutex_unlock(&sc->rx_mtx);
	}

	return (NULL);
}

static void
pci_vtnet_ping_rxq(void *vsc, struct vqueue_info *vq)
{
	struct pci_vtnet_softc *sc = vsc;

	/*
	 * A qnotify means that the rx process can now begin
	 */
	if (sc->vsc_rx_ready == 0) {
		sc->vsc_rx_ready = 1;
		vq->vq_used->vu_flags |= VRING_USED_F_NO_NOTIFY;
	}
}

static void
pci_vtnet_proctx(struct pci_vtnet_softc *sc, struct vqueue_info *vq)
{
	struct iovec iov[VTNET_MAXSEGS + 1];
	int i, n;
	int plen, tlen;
	uint16_t idx;

	/*
	 * Obtain chain of descriptors.  The first one is
	 * really the header descriptor, so we need to sum
	 * up two lengths: packet length and transfer length.
	 */
	n = vq_getchain(vq, &idx, iov, VTNET_MAXSEGS, NULL);
	assert(n >= 1 && n <= VTNET_MAXSEGS);
	plen = 0;
	tlen = (int)iov[0].iov_len;
	for (i = 1; i < n; i++) {
		plen += iov[i].iov_len;
		tlen += iov[i].iov_len;
	}

	DPRINTF(("virtio: packet send, %d bytes, %d segs\n\r", plen, n));
	pci_vtnet_tap_tx(sc, &iov[1], n - 1, plen);

	/* chain is processed, release it and set tlen */
	vq_relchain(vq, idx, (uint32_t)tlen);
}

static void
pci_vtnet_ping_txq(void *vsc, struct vqueue_info *vq)
{
	struct pci_vtnet_softc *sc = vsc;

	/*
	 * Any ring entries to process?
	 */
	if (!vq_has_descs(vq))
		return;

	/* Signal the tx thread for processing */
	pthread_mutex_lock(&sc->tx_mtx);
	vq->vq_used->vu_flags |= VRING_USED_F_NO_NOTIFY;
	if (sc->tx_in_progress == 0)
		pthread_cond_signal(&sc->tx_cond);
	pthread_mutex_unlock(&sc->tx_mtx);
}

/*
 * Thread which will handle processing of TX desc
 */
static void *
pci_vtnet_tx_thread(void *param)
{
	struct pci_vtnet_softc *sc = param;
	struct vqueue_info *vq;
	int error;

	pthread_setname_np("net:ipc:tx");

	vq = &sc->vsc_queues[VTNET_TXQ];

	/*
	 * Let us wait till the tx queue pointers get initialised &
	 * first tx signaled
	 */
	pthread_mutex_lock(&sc->tx_mtx);
	error = pthread_cond_wait(&sc->tx_cond, &sc->tx_mtx);
	assert(error == 0);

	for (;;) {
		/* note - tx mutex is locked here */
		while (sc->resetting || !vq_has_descs(vq)) {
			vq->vq_used->vu_flags &= ~VRING_USED_F_NO_NOTIFY;
			mb();
			if (!sc->resetting && vq_has_descs(vq))
				break;

			sc->tx_in_progress = 0;
			error = pthread_cond_wait(&sc->tx_cond, &sc->tx_mtx);
			assert(error == 0);
		}
		vq->vq_used->vu_flags |= VRING_USED_F_NO_NOTIFY;
		sc->tx_in_progress = 1;
		pthread_mutex_unlock(&sc->tx_mtx);

		do {
			/*
			 * Run through entries, placing them into
			 * iovecs and sending when an end-of-packet
			 * is found
			 */
			pci_vtnet_proctx(sc, vq);
		} while (vq_has_descs(vq));

		/*
		 * Generate an interrupt if needed.
		 */
		vq_endchains(vq, 1);

		pthread_mutex_lock(&sc->tx_mtx);
	}
}

#ifdef notyet
static void
pci_vtnet_ping_ctlq(void *vsc, struct vqueue_info *vq)
{
	DPRINTF(("vtnet: control qnotify!\n\r"));
}
#endif

static int
pci_vtnet_init(struct pci_devinst *pi, char *opts)
{
	struct pci_vtnet_softc *sc;
	int mac_provided;
	pthread_t sthrd;

	sc = calloc(1, sizeof(struct pci_vtnet_softc));
	pthread_mutex_init(&sc->vsc_mtx, NULL);

	vi_softc_linkup(&sc->vsc_vs, &vtnet_vi_consts, sc, pi, sc->vsc_queues);
	sc->vsc_vs.vs_mtx = &sc->vsc_mtx;

	sc->vsc_queues[VTNET_RXQ].vq_qsize = VTNET_RINGSZ;
	sc->vsc_queues[VTNET_RXQ].vq_notify = pci_vtnet_ping_rxq;
	sc->vsc_queues[VTNET_TXQ].vq_qsize = VTNET_RINGSZ;
	sc->vsc_queues[VTNET_TXQ].vq_notify = pci_vtnet_ping_txq;
#ifdef notyet
	sc->vsc_queues[VTNET_CTLQ].vq_qsize = VTNET_RINGSZ;
        sc->vsc_queues[VTNET_CTLQ].vq_notify = pci_vtnet_ping_ctlq;
#endif

	/*
	 * Attempt to open the tap device and read the MAC address
	 * if specified
	 */
	mac_provided = 0;

	if (vpnkit_create(sc, opts) == -1) {
		return (-1);
	}

	sc->vsc_config.mac[0] = sc->state->vif.mac[0];
	sc->vsc_config.mac[1] = sc->state->vif.mac[1];
	sc->vsc_config.mac[2] = sc->state->vif.mac[2];
	sc->vsc_config.mac[3] = sc->state->vif.mac[3];
	sc->vsc_config.mac[4] = sc->state->vif.mac[4];
	sc->vsc_config.mac[5] = sc->state->vif.mac[5];

	/* initialize config space */
	pci_set_cfgdata16(pi, PCIR_DEVICE, VIRTIO_DEV_NET);
	pci_set_cfgdata16(pi, PCIR_VENDOR, VIRTIO_VENDOR);
	pci_set_cfgdata8(pi, PCIR_CLASS, PCIC_NETWORK);
	pci_set_cfgdata16(pi, PCIR_SUBDEV_0, VIRTIO_TYPE_NET);
	pci_set_cfgdata16(pi, PCIR_SUBVEND_0, VIRTIO_VENDOR);

	/* Link is up if we managed to open tap device. */
	sc->vsc_config.status = 1;

	/* use BAR 1 to map MSI-X table and PBA, if we're using MSI-X */
	if (vi_intr_init(&sc->vsc_vs, 1, fbsdrun_virtio_msix()))
		return (1);

	/* use BAR 0 to map config regs in IO space */
	vi_set_io_bar(&sc->vsc_vs, 0);

	sc->resetting = 0;

	sc->rx_merge = 1;
	sc->rx_vhdrlen = sizeof(struct virtio_net_rxhdr);
	sc->rx_in_progress = 0;
	pthread_mutex_init(&sc->rx_mtx, NULL);

	/*
	 * Initialize tx semaphore & spawn TX processing thread.
	 * As of now, only one thread for TX desc processing is
	 * spawned.
	 */
	sc->tx_in_progress = 0;
	pthread_mutex_init(&sc->tx_mtx, NULL);
	pthread_cond_init(&sc->tx_cond, NULL);
	pthread_create(&sc->tx_tid, NULL, pci_vtnet_tx_thread, (void *)sc);

	if (pthread_create(&sthrd, NULL, pci_vtnet_tap_select_func, sc)) {
		fprintf(stderr, "Could not create select()-based receive thread\n");
	}

	return (0);
}

static int
pci_vtnet_cfgwrite(void *vsc, int offset, int size, uint32_t value)
{
	struct pci_vtnet_softc *sc = vsc;
	void *ptr;

	if (offset < 6) {
		assert(offset + size <= 6);
		/*
		 * The driver is allowed to change the MAC address
		 */
		ptr = &sc->vsc_config.mac[offset];
		memcpy(ptr, &value, size);
	} else {
		/* silently ignore other writes */
		DPRINTF(("vtnet: write to readonly reg %d\n\r", offset));
	}

	return (0);
}

static int
pci_vtnet_cfgread(void *vsc, int offset, int size, uint32_t *retval)
{
	struct pci_vtnet_softc *sc = vsc;
	void *ptr;

	ptr = (uint8_t *)&sc->vsc_config + offset;
	memcpy(retval, ptr, size);
	return (0);
}

static void
pci_vtnet_neg_features(void *vsc, uint64_t negotiated_features)
{
	struct pci_vtnet_softc *sc = vsc;

	sc->vsc_features = negotiated_features;

	if (!(sc->vsc_features & VIRTIO_NET_F_MRG_RXBUF)) {
		sc->rx_merge = 0;
		/* non-merge rx header is 2 bytes shorter */
		sc->rx_vhdrlen -= 2;
	}
}

static struct pci_devemu pci_de_vnet_ipc = {
	.pe_emu = 	"virtio-vpnkit",
	.pe_init =	pci_vtnet_init,
	.pe_barwrite =	vi_pci_write,
	.pe_barread =	vi_pci_read
};
PCI_EMUL_SET(pci_de_vnet_ipc);
