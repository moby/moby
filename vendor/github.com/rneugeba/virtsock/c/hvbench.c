/*
 * A Hyper-V socket benchmarking program
 */
#include "compat.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* 3049197C-FACB-11E6-BD58-64006A7986D3 */
DEFINE_GUID(BM_GUID,
    0x3049197c, 0xfacb, 0x11e6, 0xbd, 0x58, 0x64, 0x00, 0x6a, 0x79, 0x86, 0xd3);
#define BM_PORT 0x3049197c

#ifdef _MSC_VER
static WSADATA wsaData;
#endif

#ifndef ARRAY_SIZE
#define ARRAY_SIZE(_arr) (sizeof(_arr)/sizeof(*(_arr)))
#endif

/* Use a static buffer for send and receive. */
#define MAX_BUF_LEN (2 * 1024 * 1024)
static char buf[MAX_BUF_LEN];

/* Time (in ns) to run eeach bandwidth test */
#define BM_BW_TIME (10ULL * 1000 * 1000 * 1000)

/* How many connections to make */
#define BM_CONNS 2000

static int verbose;
#define INFO(...)                                                       \
    do {                                                                \
        if (verbose) {                                                  \
            printf(__VA_ARGS__);                                        \
            fflush(stdout);                                             \
        }                                                               \
    } while (0)
#define DBG(...)                                                        \
    do {                                                                \
        if (verbose > 1) {                                              \
            printf(__VA_ARGS__);                                        \
            fflush(stdout);                                             \
        }                                                               \
    } while (0)
#define TRC(...)                                                        \
    do {                                                                \
        if (verbose > 2) {                                              \
            printf(__VA_ARGS__);                                        \
            fflush(stdout);                                             \
        }                                                               \
    } while (0)


enum benchmark {
    BM_BW_UNI = 1, /* Uni-directional Bandwidth benchamrk */
    BM_LAT = 2,    /* Message ping-pong latency over single connection */
    BM_CONN = 3,   /* Connection benchmark */
};


/* There's anecdotal evidence that a blocking send()/recv() is slower
 * than performing non-blocking send()/recv() calls and then use
 * epoll()/WSAPoll().  This flags switches between the two
 */
static int opt_poll;
/* Use the vsock interface on Linux */
static int opt_vsock;


/* Bandwidth tests:
 *
 * The TX side sends a fixed amount of data in fixed sized
 * messages. The RX side drains the ring in message sized chunks (or less).
 */
static int bw_rx(SOCKET fd, int msg_sz)
{
    struct pollfd pfd = { 0 };
    int rx_sz;
    int ret;

    if (opt_poll) {
        pfd.fd = fd;
        pfd.events = POLLIN;
    }

    rx_sz = msg_sz ? msg_sz : ARRAY_SIZE(buf);

    DBG("bw_rx: msg_sz=%d rx_sz=%d\n", msg_sz, rx_sz);

    for (;;) {
        ret = recv(fd, buf, rx_sz, 0);
        if (ret == 0) {
            break;
        } else if (ret == SOCKET_ERROR) {
            if (opt_poll && poll_check()) {
                pfd.revents = 0;
                poll(&pfd, 1, -1); /* XXX no error checking */
                continue;
            }
            sockerr("recv()");
            ret = -1;
            goto err_out;
        }
        TRC("Received: %d\n", ret);
    }
    ret = 0;

err_out:
    return ret;
}

static int bw_tx(SOCKET fd, int msg_sz, uint64_t *bw)
{
    struct pollfd pfd = { 0 };
    uint64_t start, end, diff;
    int msgs_sent = 0;
    int tx_sz;
    int sent;
    int ret;

    if (opt_poll) {
        pfd.fd = fd;
        pfd.events = POLLOUT;
    }

    tx_sz = msg_sz ? msg_sz : ARRAY_SIZE(buf);

    DBG("bw_tx: msg_sz=%d tx_sz=%d \n", msg_sz, tx_sz);

    start = time_ns();
    end = time_ns();

    while (end < start + BM_BW_TIME) {
        sent = 0;
        while (sent < tx_sz) {
            ret = send(fd, buf + sent, tx_sz - sent, 0);
            if (ret == SOCKET_ERROR) {
                if (opt_poll && poll_check()) {
                    pfd.revents = 0;
                    poll(&pfd, 1, -1);  /* XXX no error checking */
                    continue;
                }
                sockerr("send()");
                ret = -1;
                goto err_out;
            }
            sent += ret;
            TRC("Sent: %d %d\n", sent, ret);
        }
        msgs_sent++;
        if (!(msgs_sent % 1000))
            end = time_ns();
    }
    DBG("bw_tx: %d %"PRIu64" %"PRIu64"\n", msgs_sent, start, end);

    /* Bandwidth in Mbits per second */
    diff = end - start;
    diff /= 1000 * 1000; /* Time in milliseconds */
    *bw = (8ULL * msgs_sent * msg_sz * 1000) / (diff * 1024 * 1024);
    ret = 0;

err_out:
    return ret;
}


/*
 * Main server and client entry points
 */
static int server(int bm, int msg_sz)
{
    SOCKET lsock, csock;
    SOCKADDR_VM savm, sacvm;
    SOCKADDR_HV sahv, sachv;
    socklen_t socklen;
    int max_conn;
    int ret = 0;

    INFO("server: bm=%d msg_sz=%d\n", bm, msg_sz);

    if (opt_vsock)
        lsock = socket(AF_VSOCK, SOCK_STREAM, 0);
    else
        lsock = socket(AF_HYPERV, SOCK_STREAM, HV_PROTOCOL_RAW);
    if (lsock == INVALID_SOCKET) {
        sockerr("socket()");
        return 1;
    }

    memset(&savm, 0, sizeof(savm));
    savm.Family = AF_VSOCK;
    savm.SvmPort = BM_PORT;
    savm.SvmCID = VMADDR_CID_ANY; /* Ignore target here */

    memset(&sahv, 0, sizeof(sahv));
    sahv.Family = AF_HYPERV;
    sahv.VmId = HV_GUID_WILDCARD;
    sahv.ServiceId = BM_GUID;

    if (opt_vsock)
        ret = bind(lsock, (const struct sockaddr *)&savm, sizeof(savm));
    else
        ret = bind(lsock, (const struct sockaddr *)&sahv, sizeof(sahv));
    if (ret == SOCKET_ERROR) {
        sockerr("bind()");
        closesocket(lsock);
        return 1;
    }

    ret = listen(lsock, SOMAXCONN);
    if (ret == SOCKET_ERROR) {
        sockerr("listen()");
        goto err_out;
    }

    INFO("server: listening\n");

    if (bm == BM_CONN)
        max_conn = BM_CONNS;
    else
        max_conn = 1;

    while (max_conn) {
        max_conn--;

        memset(&sacvm, 0, sizeof(sacvm));
        memset(&sachv, 0, sizeof(sachv));
        if (opt_vsock) {
            socklen = sizeof(sacvm);
            csock = accept(lsock, (struct sockaddr *)&sacvm, &socklen);
        } else {
            socklen = sizeof(sachv);
            csock = accept(lsock, (struct sockaddr *)&sachv, &socklen);
        }
        if (csock == INVALID_SOCKET) {
            sockerr("accept()");
            ret = -1;
            continue;
        }

        INFO("server: accepted\n");

        /* Switch to non-blocking if we want to poll */
        if (opt_poll)
            poll_enable(csock);

        ret = bw_rx(csock, msg_sz);

        closesocket(csock);
    }

err_out:
    closesocket(lsock);
    return ret;
}


static int client(GUID target, int bm, int msg_sz)
{
    SOCKET fd;
    SOCKADDR_VM savm;
    SOCKADDR_HV sahv;
    uint64_t res;
    int ret = 0;

    INFO("client: bm=%d msg_sz=%d\n", bm, msg_sz);

    if (opt_vsock)
        fd = socket(AF_VSOCK, SOCK_STREAM, 0);
    else
        fd = socket(AF_HYPERV, SOCK_STREAM, HV_PROTOCOL_RAW);
    if (fd == INVALID_SOCKET) {
        sockerr("socket()");
        return 1;
    }

    memset(&sahv, 0, sizeof(sahv));
    savm.Family = AF_VSOCK;
    savm.SvmPort = BM_PORT;
    savm.SvmCID = VMADDR_CID_ANY; /* Ignore target here */

    memset(&sahv, 0, sizeof(sahv));
    sahv.Family = AF_HYPERV;
    sahv.VmId = target;
    sahv.ServiceId = BM_GUID;

    if (opt_vsock)
        ret = connect(fd, (const struct sockaddr *)&savm, sizeof(savm));
    else
        ret = connect(fd, (const struct sockaddr *)&sahv, sizeof(sahv));
    if (ret == SOCKET_ERROR) {
        sockerr("connect()");
        ret = -1;
        goto err_out;
    }

    INFO("client: connected\n");

    /* Switch to non-blocking if we want to poll */
    if (opt_poll)
        poll_enable(fd);

    if (bm == BM_BW_UNI) {
        ret = bw_tx(fd, msg_sz, &res);
        if (ret)
            goto err_out;
        printf("%d %"PRIu64"\n", msg_sz, res);
    } else {
        fprintf(stderr, "Unknown benchmark %d\n", bm);
        ret = -1;
    }

err_out:
    closesocket(fd);
    return ret;
}

/* Different client for connection tests */
#define BM_CONN_TIMEOUT 500 /* 500ms */
static int client_conn(GUID target)
{
    uint64_t start, end, diff;
    int histogram[3 * 9 + 3];
    SOCKADDR_VM savm;
    SOCKADDR_HV sahv;
    SOCKET fd;
    int sum;
    int ret;
    int i;

    memset(histogram, 0, sizeof(histogram));

    INFO("client: connection test\n");

    for (i = 0; i < BM_CONNS; i++) {
        if (opt_vsock)
            fd = socket(AF_VSOCK, SOCK_STREAM, 0);
        else
            fd = socket(AF_HYPERV, SOCK_STREAM, HV_PROTOCOL_RAW);
        if (fd == INVALID_SOCKET) {
            histogram[ARRAY_SIZE(histogram) - 1] += 1;
            DBG("conn: %d -> socket error\n", i);
            continue;
        }

        memset(&sahv, 0, sizeof(sahv));
        savm.Family = AF_VSOCK;
        savm.SvmPort = BM_PORT;
        savm.SvmCID = VMADDR_CID_ANY; /* Ignore target here */

        memset(&sahv, 0, sizeof(sahv));
        sahv.Family = AF_HYPERV;
        sahv.VmId = target;
        sahv.ServiceId = BM_GUID;

        start = time_ns();

        if (opt_poll)
            if (opt_vsock)
                ret = connect_ex(fd, (const struct sockaddr *)&savm, sizeof(savm),
                                 BM_CONN_TIMEOUT);
            else
                ret = connect_ex(fd, (const struct sockaddr *)&sahv, sizeof(sahv),
                                 BM_CONN_TIMEOUT);
        else
            if (opt_vsock)
                ret = connect(fd, (const struct sockaddr *)&savm, sizeof(savm));
            else
                ret = connect(fd, (const struct sockaddr *)&sahv, sizeof(sahv));

        if (ret == SOCKET_ERROR) {
            histogram[ARRAY_SIZE(histogram) - 2] += 1;
            DBG("conn: %d -> connect error\n", i);
        } else {
            end = time_ns();
            diff = (end - start);
            DBG("conn: %d -> %"PRIu64"ns\n", i, diff);

            diff /= (1000 * 1000);
            if (diff < 10)
                histogram[diff] += 1;
            else if (diff < 100)
                histogram[9 + diff / 10] += 1;
            else if (diff < 1000)
                histogram[18 + diff / 100] += 1;
            else
                histogram[ARRAY_SIZE(histogram) - 3] += 1;
        }

        closesocket(fd);
    }

    /* Print the results */
    printf("# time (ms) vs count vs cumulative percent\n");
    sum = 0;
    for (i = 0; i < ARRAY_SIZE(histogram); i++) {
        sum += histogram[i];
        if (i < 9)
            printf("%d %d %6.2f\n", i + 1, histogram[i],
                   sum * 100.0 / BM_CONNS);
        else if (i < 18)
            printf("%d %d %6.2f\n", (i - 9 + 1) * 10, histogram[i],
                   sum * 100.0 / BM_CONNS);
        else if (i < 27)
            printf("%d %d %6.2f\n", (i - 18 + 1) * 100, histogram[i],
                   sum * 100.0 / BM_CONNS);
        else if (i == ARRAY_SIZE(histogram) - 3)
            printf(">=%d %d %6.2f\n", (i - 27 + 1) * 1000, histogram[i],
                   sum * 100.0 / BM_CONNS);
        else if (i == ARRAY_SIZE(histogram) - 2)
            printf("connect_err %d %6.2f\n", histogram[i],
                   sum * 100.0 / BM_CONNS);
        else
            printf("socket_err %d %6.2f\n", histogram[i],
                   sum * 100.0 / BM_CONNS);
    }

    return 0;
}

void usage(char *name)
{
    printf("%s: -s|-c <carg> -b|-l -m <sz> [-v]\n", name);
    printf(" -s        Server mode\n");
    printf(" -c <carg> Client mode. <carg>:\n");
    printf("   'loopback': Connect in loopback mode\n");
    printf("   'parent':   Connect to the parent partition\n");
    printf("   <guid>:     Connect to VM with GUID\n");
    printf("\n");
    printf(" -B        Bandwidth test\n");
    printf(" -L        Latency test\n");
    printf(" -C        Connection test\n");
    printf("\n");
    printf(" -vsock    Use vsock (Linux only)\n");
    printf(" -m <sz>   Message size in bytes\n");
    printf(" -p        Use poll instead of blocking send()/recv()\n");
    printf(" -v        Verbose output\n");
}

int __cdecl main(int argc, char **argv)
{
    int opt_server = 0;
    int opt_bm = 0;
    int opt_msgsz = 0;
    GUID target;
    int res = 0;
    int i;

#ifdef _MSC_VER
    /* Initialize Winsock */
    res = WSAStartup(MAKEWORD(2,2), &wsaData);
    if (res != 0) {
        fprintf(stderr, "WSAStartup() failed with error: %d\n", res);
        return 1;
    }
#endif

    /* No getopt on windows. Do some manual parsing */
    for (i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-s") == 0) {
            opt_server = 1;
        } else if (strcmp(argv[i], "-c") == 0) {
            opt_server = 0;
            if (i + 1 >= argc) {
                fprintf(stderr, "-c requires an argument\n");
                usage(argv[0]);
                goto out;
            }
            if (strcmp(argv[i + 1], "loopback") == 0) {
                target = HV_GUID_LOOPBACK;
            } else if (strcmp(argv[i + 1], "parent") == 0) {
                target = HV_GUID_PARENT;
            } else {
                res = parseguid(argv[i + 1], &target);
                if (res != 0) {
                    fprintf(stderr, "failed to scan: %s\n", argv[i + 1]);
                    goto out;
                }
            }
            i++;

        } else if (strcmp(argv[i], "-B") == 0) {
            opt_bm = BM_BW_UNI;
        } else if (strcmp(argv[i], "-L") == 0) {
            opt_bm = BM_LAT;
        } else if (strcmp(argv[i], "-C") == 0) {
            opt_bm = BM_CONN;

        } else if (strcmp(argv[i], "-vsock") == 0) {
            opt_vsock = 1;
        } else if (strcmp(argv[i], "-m") == 0) {
            if (i + 1 >= argc) {
                fprintf(stderr, "-m requires an argument\n");
                usage(argv[0]);
                goto out;
            }
            opt_msgsz = atoi(argv[++i]);
        } else if (strcmp(argv[i], "-p") == 0) {
            opt_poll = 1;
        } else if (strcmp(argv[i], "-v") == 0) {
            verbose++;
        } else {
            usage(argv[0]);
            goto out;
        }
    }

#ifdef _MSC_VER
    if (opt_vsock) {
        fprintf(stderr, "-vsock is not valid on Windows\n");
        goto out;
    }
#endif

    if (!opt_bm) {
        fprintf(stderr, "You need to specify a test\n");
        goto out;
    }
    if (opt_bm == BM_LAT) {
        fprintf(stderr, "Latency tests currently not implemented\n");
        goto out;
    }

    if (opt_server) {
        res = server(opt_bm, opt_msgsz);
    } else {
        if (opt_bm == BM_CONN)
            res = client_conn(target);
        else
            res = client(target, opt_bm, opt_msgsz);
    }

out:
#ifdef _MSC_VER
    WSACleanup();
#endif
    return res;
}
