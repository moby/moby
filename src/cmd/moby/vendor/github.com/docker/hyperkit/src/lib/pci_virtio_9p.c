#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <pthread.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <assert.h>
#include <sys/param.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/uio.h>
#include <arpa/inet.h>
#include <xhyve/support/misc.h>
#include <xhyve/support/linker_set.h>
#include <xhyve/xhyve.h>
#include <xhyve/pci_emul.h>
#include <xhyve/virtio.h>

#define VIRTIO_9P_MOUNT_TAG 1

static int pci_vt9p_debug = 0;
#define DPRINTF(params) if (pci_vt9p_debug) printf params

/* XXX issues with larger buffers elsewhere in stack */
#define BUFSIZE (1 << 18)
#define MAXDESC (BUFSIZE / 4096 + 4)
#define VT9P_RINGSZ (BUFSIZE / 4096 * 4)

struct virtio_9p_config {
	uint16_t tag_len;
	uint8_t tag[256];
};
/*
 * Per-device softc
 */
struct pci_vt9p_out {
	struct iovec wiov[MAXDESC];
	struct vqueue_info *vq;
	int inuse;
	uint16_t tag;
	uint16_t idx;
	uint16_t otag;
};
struct pci_vt9p_softc {
	struct virtio_softc v9sc_vs;
	struct vqueue_info v9sc_vq;
	pthread_mutex_t v9sc_mtx;
	pthread_mutex_t v9sc_mtx2;
	pthread_t v9sc_thread;
	struct virtio_9p_config v9sc_cfg;
	struct pci_vt9p_out v9sc_out[VT9P_RINGSZ];
	/* -1 means not connected yet */
	int v9sc_sock;
	int v9sc_inflight;
	int port;
	char *path;
};

static void pci_vt9p_reset(void *);
static void pci_vt9p_notify(void *, struct vqueue_info *);
static int pci_vt9p_cfgread(void *, int, int, uint32_t *);
static int pci_vt9p_cfgwrite(void *, int, int, uint32_t);
static void pci_vt9p_lazy_initialise_socket(struct pci_vt9p_softc *sc);
static void *pci_vt9p_thread(void *vsc);

static struct virtio_consts vt9p_vi_consts = {
	"vt9p", /* our name */
	1, /* we support 1 virtqueue */
	0, /* config reg size */
	pci_vt9p_reset, /* reset */
	pci_vt9p_notify, /* device-wide qnotify */
	pci_vt9p_cfgread, /* read virtio config */
	pci_vt9p_cfgwrite, /* write virtio config */
	NULL, /* apply negotiated features */
	VIRTIO_9P_MOUNT_TAG, /* our capabilities */
};

static void
pci_vt9p_reset(void *vsc)
{
	struct pci_vt9p_softc *sc;

	sc = vsc;

	DPRINTF(("vt9p: device reset requested !\n"));
	vi_reset_dev(&sc->v9sc_vs);
}

/* Used to lazily initialise the socket */
static void
pci_vt9p_lazy_initialise_socket(struct pci_vt9p_softc *sc)
{
	struct sockaddr_in sa_in;
	struct sockaddr_un sa_un;
	int so;
	socklen_t sol = (socklen_t) sizeof(int);

	if (sc->v9sc_sock != -1)
		return;
	sa_in.sin_family = AF_INET;
	sa_in.sin_port = htons(sc->port);
	sa_in.sin_addr.s_addr = htonl(INADDR_LOOPBACK);

	sa_un.sun_family = AF_UNIX;
	memset(&sa_un, 0, sizeof(sa_un));
	strncpy(sa_un.sun_path, sc->path, sizeof(sa_un.sun_path)-1);

	int domain = (sc->port != -1)?AF_INET:AF_UNIX;
	struct sockaddr *sa = (sc->port != -1)?(struct sockaddr*)&sa_in:(struct sockaddr*)&sa_un;
	size_t sa_len = (sc->port != -1)?sizeof(sa_in):sizeof(sa_un);

	int max_attempts = 200; /* 200 * 50ms = 10s */
	do {
		sc->v9sc_sock = socket(domain, SOCK_STREAM, 0);
		if (sc->v9sc_sock == -1) {
			if (sc->port != -1) {
				fprintf(stderr, "virtio-9p: failed to connect to port %d: out of file descriptors\n", sc->port);
			} else {
				fprintf(stderr, "virtio-9p: failed to connect to path %s: out of file descriptors\n", sc->path);
			}
			/* The device won't work */
			return;
		}

		if (connect(sc->v9sc_sock, sa, (socklen_t)sa_len) == -1) {
			close(sc->v9sc_sock);
			sc->v9sc_sock = -1;
			usleep(50000);
		}
	} while ((sc->v9sc_sock == -1) && (--max_attempts > 0));

	if (sc->v9sc_sock == -1) {
		if (sc->port != -1) {
			fprintf(stderr, "virtio-9p: failed to connect to port %d\n", sc->port);
		} else {
			fprintf(stderr, "virtio-9p: failed to connect to path %s\n", sc->path);
		}
		/* The device won't work */
	}
	if (getsockopt(sc->v9sc_sock, SOL_SOCKET, SO_SNDBUF, &so, &sol) != -1) {
		if (so < 2 * BUFSIZE) {
			so = 2 * BUFSIZE;
		(void)setsockopt(sc->v9sc_sock, SOL_SOCKET, SO_SNDBUF, &so, sol);
		(void)setsockopt(sc->v9sc_sock, SOL_SOCKET, SO_RCVBUF, &so, sol);
		}
	}
	if (pthread_create(&sc->v9sc_thread, NULL, pci_vt9p_thread, sc) == -1) {
		perror("pthread_create");
		/* The device won't work */
	}
}

static void
pci_vt9p_notify(void *vsc, struct vqueue_info *vq)
{
	struct iovec iov[MAXDESC];
	uint16_t flags[MAXDESC];
	struct pci_vt9p_softc *sc = vsc;
	uint16_t idx;
	ssize_t n;
	int nvec, i, freevec;
	struct iovec *wiov;
	int nread, nwrite;
	size_t readbytes;
	uint16_t tag;
	uint32_t len;
	uint8_t command;
	uint16_t otag = 0;
	int used = 0;

	sc = vsc;

	pci_vt9p_lazy_initialise_socket(sc);

	/* will be a socket here */
	if (sc->v9sc_sock < 0) {
		DPRINTF(("vt9p socket invalid\r\n"));
		vq_endchains(vq, 0);
			return;
	}

	nvec = vq_getchain(vq, &idx, iov, MAXDESC, flags);

	if (nvec == -1) {
		DPRINTF(("vt9p bad descriptors\r\n"));
		return; /* what to do? */
	}

	if (nvec == 0) {
		DPRINTF(("vt9p got all the descriptors\r\n"));
		return;
	}
	DPRINTF(("vt9p got %d descriptors\r\n", nvec));

	wiov = NULL;
	nwrite = 0;
	nread = 0;
	readbytes = 0;
	tag = 0;
	freevec = -1;

	DPRINTF(("vtrnd: vt9p_notify(): %d count %d\r\n", (int)idx, nvec));
	for (i = 0; i < nvec; i++) {
		DPRINTF(("vt9p iovec %d len %d\r\n", i, (int)iov[i].iov_len));
		if (flags[i] & VRING_DESC_F_WRITE) {
			DPRINTF(("writeable\r\n"));
			nwrite++;
		} else {
			if (nwrite == 0) {
				nread++;
				readbytes += iov[i].iov_len;
				DPRINTF(("readable\r\n"));
			} else {
				DPRINTF(("ignoring readable buffers after writeable ones\r\n"));
			}
		}
		if (wiov == NULL && (flags[i] & VRING_DESC_F_WRITE)) {
			wiov = &iov[i];
			DPRINTF(("vt9p wiov is %p\r\n", (void*)wiov));
		}
	}
	/* do this properly */
	if (iov[0].iov_len >= 7) {
		uint8_t *ptr = (uint8_t *)iov[0].iov_base;
		len = (uint32_t)ptr[0] | ((uint32_t)ptr[1] << 8) | ((uint32_t)ptr[2] << 16) | ((uint32_t)ptr[3] << 24);
		command = ptr[4];
		tag = (uint16_t)((uint16_t)ptr[5] | ((uint16_t)ptr[6] << 8));
		DPRINTF(("vt9p len %d\r\n", (int)len));
		DPRINTF(("vt9p command %d\r\n", (int)command));
		DPRINTF(("vt9p tag %d\r\n", (int)tag));
		otag = 0;
		if (command == 108 && iov[0].iov_len >= 9) {
			otag = (uint16_t)((uint16_t)ptr[7] | ((uint16_t)ptr[8] << 8));
			DPRINTF(("TFlush otag %d\r\n", (int)otag));
		}
		/* Linux is buggy with writes over 1k, has a buggy zero copy codepath, fix up */
		if (command == 118 && iov[0].iov_len >= 23) {
			uint32_t wlen = (uint32_t)ptr[19] | ((uint32_t)ptr[20] << 8) | ((uint32_t)ptr[21] << 16) | ((uint32_t)ptr[22] << 24);
			DPRINTF(("Twrite wlen %d readbytes %d len %d\r\n", (int)wlen, (int)readbytes, (int)len));
			if (readbytes != len) {
				DPRINTF(("FIXUP! len\n"));
				ptr[0] = (uint8_t)(readbytes & 0xff);
				ptr[1] = (uint8_t)((readbytes >> 8) & 0xff);
				ptr[2] = (uint8_t)((readbytes >> 16) & 0xff);
				ptr[3] = (uint8_t)((readbytes >> 24) & 0xff);
			}
			/* XXX not sure seeing this now */
			if (wlen != readbytes - 23) {
				DPRINTF(("FIXUP! wlen\n"));
				wlen = (uint32_t) (readbytes - 23);
				ptr[19] = (uint8_t)(wlen & 0xff);
				ptr[20] = (uint8_t)((wlen >> 8) & 0xff);
				ptr[21] = (uint8_t)((wlen >> 16) & 0xff);
				ptr[22] = (uint8_t)((wlen >> 24) & 0xff);
			}
		}
	} else {
		DPRINTF(("vt9p oops split iovec for command - do this properly\r\n"));
	}

	if (nwrite == 0) {
		DPRINTF(("Nowhere to write to!!\r\n"));
	}
	/* do something with request! */
	pthread_mutex_lock(&sc->v9sc_mtx2);
	sc->v9sc_inflight++;
	for (i = 0; i < VT9P_RINGSZ; i++) {
		if (sc->v9sc_out[i].inuse == 1) {
			used++;
			continue;
		}
		sc->v9sc_out[i].inuse = 1;
		memcpy(sc->v9sc_out[i].wiov, wiov, (size_t)(sizeof(struct iovec) * (size_t)nwrite));
		sc->v9sc_out[i].vq = vq;
		sc->v9sc_out[i].tag = tag;
		sc->v9sc_out[i].idx = idx;
		sc->v9sc_out[i].otag = otag;
		break;
	}
	if (used == VT9P_RINGSZ) {
		fprintf(stderr, "virtio-9p: Ring full!\n");
		_exit(1);
	}
	pthread_mutex_unlock(&sc->v9sc_mtx2);

	i = 0;
	while (readbytes) {
		n = writev(sc->v9sc_sock, &iov[i], nread);
		if (n <= 0) {
			fprintf(stderr, "virtio-9p: unexpected EOF writing to server-- did the 9P server crash?\n");
			/* Fatal error, crash VM, let us be restarted */
			_exit(1);
		}
		DPRINTF(("vt9p: wrote to sock %d bytes\r\n", (int)n));
		readbytes -= (size_t)n;
		if (readbytes != 0) {
			while ((size_t)n >= iov[i].iov_len) {
				n -= iov[i].iov_len;
				i++;
			}
			iov[i].iov_len -= (size_t) n;
			iov[i].iov_base = (char *) iov[i].iov_base + n;
		}
	}
}

static void *
pci_vt9p_thread(void *vsc)
{
	struct pci_vt9p_softc *sc = vsc;
	ssize_t ret;
	size_t n;
	size_t minlen = 7;
	uint32_t len;
	uint16_t tag, otag;
	uint8_t command;
	uint8_t *ptr;
	int i, ii, j;
	struct iovec *wiov;
	uint8_t *buf;
	char ident[16];

	snprintf(ident, sizeof(ident), "9p:%s", sc->v9sc_cfg.tag);
	pthread_setname_np(ident);

	buf = calloc(1, BUFSIZE);
	if (! buf) {
		fprintf(stderr, "virtio-p9: memory allocation failed\n");
		_exit(1);
	}

	while (1) {
		ptr = buf;
		n = 0;
		while (n < minlen) {
			ret = read(sc->v9sc_sock, ptr, minlen - n);
			if (ret <= 0) {
				fprintf(stderr, "virtio-9p: unexpected EOF reading -- did the 9P server crash?\n");
				/* Fatal error, crash VM, let us be restarted */
				_exit(1);
			}
			n += (size_t) ret;
			ptr += ret;
		}
		len = (uint32_t)buf[0] | ((uint32_t)buf[1] << 8) | ((uint32_t)buf[2] << 16) | ((uint32_t)buf[3] << 24);
		command = buf[4];
		tag = (uint16_t)((uint16_t)buf[5] | ((uint16_t)buf[6] << 8));
		DPRINTF(("[thread]Got response for tag %d command %d len %d\r\n", (int)tag, (int)command, (int)len));
		n = (size_t)(len - minlen);
		ptr = buf + minlen;
		while (n) {
			assert(len <= BUFSIZE);
			ret = read(sc->v9sc_sock, ptr, n);
			if (ret <= 0) {
				fprintf(stderr, "virtio-9p: unexpected EOF reading-- did the 9P server crash?\n");
				/* Fatal error, crash the VM, let us be restarted */
				_exit(1);
			}
			n -= (size_t) ret;
			ptr += ret;
		}
		DPRINTF(("[thread]got complete response for tag %d len %d\r\n", (int)tag, (int)len));
		if (command == 107) {
			char msg[128];
			uint16_t slen = (uint16_t)((uint16_t)buf[7] | ((uint16_t)buf[8] << 8));
			memcpy(msg, &buf[9], slen);
			msg[slen] = 0;
			DPRINTF(("[thread]Rerror: %s\r\n", msg));
		}
		if (command == 109) { /* Rflush */
			for (i = 0; i < VT9P_RINGSZ; i++) {
				if (sc->v9sc_out[i].tag == tag) {
					otag = sc->v9sc_out[i].otag;
					for (j = 0; j < VT9P_RINGSZ; j++) {
						if (sc->v9sc_out[j].tag == otag && sc->v9sc_out[j].inuse) {
							pthread_mutex_lock(&sc->v9sc_mtx2);
							sc->v9sc_out[j].inuse = 0;
							sc->v9sc_inflight--;
							vq_relchain(sc->v9sc_out[j].vq, sc->v9sc_out[j].idx, ((uint32_t) 0));
							pthread_mutex_unlock(&sc->v9sc_mtx2);
							break;
						}
					}
					break;
				}
			}
		}
		for (i = 0; i < VT9P_RINGSZ; i++) {
			if (sc->v9sc_out[i].tag == tag) {
				wiov = sc->v9sc_out[i].wiov;
				ii = 0;
				ptr = buf;
				n = len;
				while (n) {
					size_t m = n;
					if (m > wiov[ii].iov_len)
						m = wiov[ii].iov_len;
					DPRINTF(("[thread]copy %d bytes to iov at %p\r\n", (int)m, wiov[ii].iov_base));
					memcpy(wiov[ii].iov_base, ptr, m);
					ptr += m;
					n -= (size_t)m;
					ii++;
				}
				DPRINTF(("[thread]release\r\n"));
				pthread_mutex_lock(&sc->v9sc_mtx2);
				vq_relchain(sc->v9sc_out[i].vq, sc->v9sc_out[i].idx, ((uint32_t) len));
				sc->v9sc_out[i].inuse = 0;
				sc->v9sc_inflight--;
				/* Generate interrupt even if some requests are outstanding, because
				  if we're using a blocking poll then we expect one request to be
				  permanently outstanding at all times. */
				DPRINTF(("[thread]endchain\r\n"));
				vq_endchains(sc->v9sc_out[i].vq, 1);
				pthread_mutex_unlock(&sc->v9sc_mtx2);
				break;
			}
		}
	}

	return NULL;
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
pci_vt9p_init(struct pci_devinst *pi, char *opts)
{
	struct pci_vt9p_softc *sc;

	int port = -1; /* if != -1, the port is valid. path is valid otherwise */
	char *path = "";
	char *tag = "plan9";

	sc = calloc(1, sizeof(struct pci_vt9p_softc));
	if (! sc) {
		return 1;
	}
	sc->v9sc_sock = -1;
	fprintf(stdout, "virtio-9p: initialising %s\n", opts);

	while (1) {
		char *next;
		if (! opts)
			break;
		next = strchr(opts, ',');
		if (next)
			next[0] = '\0';
		if (strncmp(opts, "port=", 5) == 0) {
			port = atoi(&opts[5]);
			if (port == 0) {
				fprintf(stderr, "bad port: %s\r\n", opts);
				return 1;
			}
		} else if (strncmp(opts, "path=", 5) == 0) {
			path = copy_up_to_comma(opts + 5);
		} else if (strncmp(opts, "tag=", 4) == 0) {
			tag = copy_up_to_comma(opts + 4);
		} else {
			fprintf(stderr, "invalid option: %s\r\n", opts);
			return 1;
		}

		if (! next)
			break;
		opts = &next[1];
	}
	if (!((port == -1) != (strcmp(path, "") == 0))) {
		fprintf(stderr, "Please pass *either* a port *or* a path. You must pass one, you must not pass both.\n");
		return 1;
	}
	sc->port = port;
	sc->path = path;

	sc->v9sc_cfg.tag_len = (uint16_t) strlen(tag);
	if (sc->v9sc_cfg.tag_len > 256) {
		return 1;
	}
	memcpy(sc->v9sc_cfg.tag, tag, sc->v9sc_cfg.tag_len);

	pthread_mutex_init(&sc->v9sc_mtx, NULL);
	pthread_mutex_init(&sc->v9sc_mtx2, NULL);


	vi_softc_linkup(&sc->v9sc_vs, &vt9p_vi_consts, sc, pi, &sc->v9sc_vq);
	sc->v9sc_vs.vs_mtx = &sc->v9sc_mtx;

	sc->v9sc_vq.vq_qsize = VT9P_RINGSZ;

	/* initialize config space */
	pci_set_cfgdata16(pi, PCIR_DEVICE, VIRTIO_DEV_9P);
	pci_set_cfgdata16(pi, PCIR_VENDOR, VIRTIO_VENDOR);
	pci_set_cfgdata8(pi, PCIR_CLASS, PCIC_OTHER);
	pci_set_cfgdata16(pi, PCIR_SUBDEV_0, VIRTIO_TYPE_9P);
	pci_set_cfgdata16(pi, PCIR_SUBVEND_0, VIRTIO_VENDOR);

	if (vi_intr_init(&sc->v9sc_vs, 1, fbsdrun_virtio_msix()))
		return (1);
	vi_set_io_bar(&sc->v9sc_vs, 0);

	return (0);
}


static int
pci_vt9p_cfgwrite(UNUSED void *vsc, int offset, UNUSED int size,
	UNUSED uint32_t value)
{
	DPRINTF(("vt9p: write to reg %d\n\r", offset));
	return 1;
}

static int
pci_vt9p_cfgread(void *vsc, int offset, int size, uint32_t *retval)
{
	struct pci_vt9p_softc *sc = vsc;
	void *ptr;

	DPRINTF(("vt9p: read to reg %d\n\r", offset));
	ptr = (uint8_t *)&sc->v9sc_cfg + offset;
	memcpy(retval, ptr, size);

	return 0;
}


static struct pci_devemu pci_de_v9p = {
	.pe_emu =		"virtio-9p",
	.pe_init =		pci_vt9p_init,
	.pe_barwrite =	vi_pci_write,
	.pe_barread =	vi_pci_read
};
PCI_EMUL_SET(pci_de_v9p);
