/*
 * Compatibility layer between Windows and Linux
 */
#ifdef _MSC_VER

#undef UNICODE
#define WIN32_LEAN_AND_MEAN
#define _CRT_SECURE_NO_WARNINGS

#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>
#include <hvsocket.h>

#pragma comment (lib, "Ws2_32.lib")
#pragma comment (lib, "Mswsock.lib")
#pragma comment (lib, "AdvApi32.lib")

#else /* !_MSC_VER */
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdint.h>
#include <time.h>
#include <unistd.h>
#include <sys/select.h>
#include <sys/socket.h>
#endif /* !_MSC_VER */

#include <inttypes.h>

#ifdef _MSC_VER
typedef int socklen_t;

typedef __int8 int8_t;
typedef __int16 int16_t;
typedef __int32 int32_t;
typedef __int64 int64_t;

typedef unsigned __int8 uint8_t;
typedef unsigned __int16 uint16_t;
typedef unsigned __int32 uint32_t;
typedef unsigned __int64 uint64_t;
#endif

#ifndef _MSC_VER
/* Compat layer for Linux/Unix */
typedef int SOCKET;

#ifndef SOCKET_ERROR
#define SOCKET_ERROR -1
#endif

#ifndef INVALID_SOCKET
#define INVALID_SOCKET -1
#endif

#define closesocket(_fd) close(_fd)

/* Shutdown flags are different too */
#define SD_SEND    SHUT_WR
#define SD_RECEIVE SHUT_RD
#define SD_BOTH    SHUT_RDWR

#define __cdecl

/* GUID handling  */
typedef struct _GUID {
    uint32_t Data1;
    uint16_t Data2;
    uint16_t Data3;
    uint8_t  Data4[8];
} GUID;

#define DEFINE_GUID(name, l, w1, w2, b1, b2, b3, b4, b5, b6, b7, b8) \
    const GUID name = {l, w1, w2, {b1, b2,  b3,  b4,  b5,  b6,  b7,  b8}}


/* HV Socket definitions */
#define AF_HYPERV 43
#define HV_PROTOCOL_RAW 1

typedef struct _SOCKADDR_HV
{
    unsigned short Family;
    unsigned short Reserved;
    GUID VmId;
    GUID ServiceId;
} SOCKADDR_HV;

DEFINE_GUID(HV_GUID_ZERO,
    0x00000000, 0x0000, 0x0000, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00);
DEFINE_GUID(HV_GUID_BROADCAST,
    0xFFFFFFFF, 0xFFFF, 0xFFFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF);
DEFINE_GUID(HV_GUID_WILDCARD,
    0x00000000, 0x0000, 0x0000, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00);

DEFINE_GUID(HV_GUID_CHILDREN,
    0x90db8b89, 0x0d35, 0x4f79, 0x8c, 0xe9, 0x49, 0xea, 0x0a, 0xc8, 0xb7, 0xcd);
DEFINE_GUID(HV_GUID_LOOPBACK,
    0xe0e16197, 0xdd56, 0x4a10, 0x91, 0x95, 0x5e, 0xe7, 0xa1, 0x55, 0xa8, 0x38);
DEFINE_GUID(HV_GUID_PARENT,
    0xa42e7cda, 0xd03f, 0x480c, 0x9c, 0xc2, 0xa4, 0xde, 0x20, 0xab, 0xb8, 0x78);

#endif /* !_MSC_VER */

/* Common definitions (only valid on Linux, though) */
#ifndef AF_VSOCK
#define AF_VSOCK 40
#endif
#ifndef VMADDR_CID_ANY
#define VMADDR_CID_ANY -1U
#endif
typedef struct _SOCKADDR_VM
{
    unsigned short Family;
    unsigned short Reserved;
    unsigned int SvmPort;
    unsigned int SvmCID;
#ifndef _MSC_VER
    unsigned char svm_zero[sizeof(struct sockaddr) -
                           sizeof(sa_family_t) - sizeof(unsigned short) -
                           sizeof(unsigned int) - sizeof(unsigned int)];
#endif
} SOCKADDR_VM;


/* Thread wrappers */
#ifdef _MSC_VER
typedef HANDLE THREAD_HANDLE;

static inline int thread_create(THREAD_HANDLE *t, void *(*f)(void *), void *arg)
{
    *t = CreateThread(NULL, 0, (LPTHREAD_START_ROUTINE)f, arg, 0, NULL);
    return 0;
}

static inline int thread_join(THREAD_HANDLE t)
{
    WaitForSingleObject(t, INFINITE);
    return 0;
}

static inline int thread_detach(THREAD_HANDLE t)
{
    return CloseHandle(t);
}
#else
#include <pthread.h>

typedef pthread_t THREAD_HANDLE;

static inline int thread_create(THREAD_HANDLE *t, void *(*f)(void *), void *arg)
{
    return pthread_create(t, NULL, f, arg);
}

static inline int thread_join(THREAD_HANDLE t)
{
    return pthread_join(t, NULL);
}

static inline int thread_detach(THREAD_HANDLE t)
{
    return pthread_detach(t);
}
#endif


/* Time wrappers */
#ifdef _MSC_VER
static inline uint64_t time_ns(void)
{
    LARGE_INTEGER t, freq;

    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&t);

    t.QuadPart *= 1000000000;
    return (uint64_t)t.QuadPart / freq.QuadPart;
}

static inline unsigned int sleep(unsigned int sec)
{
    Sleep(sec * 1000);
    return 0;
}

#else
static inline uint64_t time_ns(void)
{
    struct timespec ts;
    int ret;

    ret = clock_gettime(CLOCK_MONOTONIC_RAW, &ts);
    if (ret)
        return 0;

    /* We don't really mind if this overflows...There are plenty of bits */
    return (uint64_t)ts.tv_sec * 1000000000 + ts.tv_nsec;
}
#endif

/*
 * Finally some common utility macros and functions
 */
#include <stdio.h>
#include <string.h>

#define GUID_FMT "%08x-%04hx-%04hx-%02x%02x-%02x%02x%02x%02x%02x%02x"
#define GUID_ARGS(_g)                                               \
    (_g).Data1, (_g).Data2, (_g).Data3,                             \
    (_g).Data4[0], (_g).Data4[1], (_g).Data4[2], (_g).Data4[3],     \
    (_g).Data4[4], (_g).Data4[5], (_g).Data4[6], (_g).Data4[7]
#define GUID_SARGS(_g)                                              \
    &(_g).Data1, &(_g).Data2, &(_g).Data3,                          \
    &(_g).Data4[0], &(_g).Data4[1], &(_g).Data4[2], &(_g).Data4[3], \
    &(_g).Data4[4], &(_g).Data4[5], &(_g).Data4[6], &(_g).Data4[7]


static inline int parseguid(const char *s, GUID *g)
{
    int res;
    int p0, p1, p2, p3, p4, p5, p6, p7;

    res = sscanf(s, GUID_FMT,
                 &g->Data1, &g->Data2, &g->Data3,
                 &p0, &p1, &p2, &p3, &p4, &p5, &p6, &p7);
    if (res != 11)
        return 1;
    g->Data4[0] = p0;
    g->Data4[1] = p1;
    g->Data4[2] = p2;
    g->Data4[3] = p3;
    g->Data4[4] = p4;
    g->Data4[5] = p5;
    g->Data4[6] = p6;
    g->Data4[7] = p7;
    return 0;
}

/* Slightly different error handling between Windows and Linux */
static inline void sockerr(const char *msg)
{
#ifdef _MSC_VER
    fprintf(stderr, "%s Error: %d\n", msg, WSAGetLastError());
#else
    fprintf(stderr, "%s Error: %d. %s\n", msg, errno, strerror(errno));
#endif
}

/* poll wrappers */

/* Set socket to non-blocking */
static inline int poll_enable(SOCKET s)
{
    int ret;
#ifdef _MSC_VER
    unsigned long mode = 1;
    ret = ioctlsocket(s, FIONBIO, &mode);
#else
    int flags;
    flags = fcntl(s, F_GETFL, 0);
    if (flags < 0)
        return flags;
    ret = fcntl(s, F_SETFL, flags | O_NONBLOCK);
#endif
    return ret;
}

/* Set socket to non-blocking */
static inline int poll_disable(SOCKET s)
{
    int ret;
#ifdef _MSC_VER
    unsigned long mode = 0;
    ret = ioctlsocket(s, FIONBIO, &mode);
#else
    int flags;
    flags = fcntl(s, F_GETFL, 0);
    if (flags < 0)
        return flags;
    ret = fcntl(s, F_SETFL, flags & ~O_NONBLOCK);
#endif
    return ret;
}

/* Return true if we should poll */
static inline int poll_check()
{
#ifdef _MSC_VER
    int err = WSAGetLastError();
    return err == WSAEWOULDBLOCK || err == WSAEFAULT;
#else
    return errno == EWOULDBLOCK || errno == EAGAIN;
#endif
}

#ifdef _MSC_VER
static inline int poll(struct pollfd fds[], unsigned long nfds, int timeout)
{
    return WSAPoll(fds, nfds, timeout);
}
#endif

/* Connect with timeout (in milliseconds), different to WinSock ConnectEx() */
static inline int connect_ex(int s, const struct sockaddr *sa,
                             socklen_t len, int timeout)
{
    struct timeval tv;
    fd_set fdset;
    int ret;

    ret = poll_enable(s);
    if (ret < 0)
        return ret;

    ret = connect(s, sa, len);
    if (!ret)
        goto out;  /* Connected */

    /* Got an error, see if we should select() */
#ifdef _MSC_VER
    ret = WSAGetLastError();
    if (ret != WSAEWOULDBLOCK) {
        ret = SOCKET_ERROR;
        goto out;
    }
#else
    if (errno != EINPROGRESS)
        goto out;
#endif

    FD_ZERO(&fdset);
    FD_SET(s, &fdset);
    tv.tv_sec = 0;
    tv.tv_usec = timeout * 1000;

    ret = select(s + 1, NULL, &fdset, NULL, &tv);
    if (ret != 1) {
        ret = SOCKET_ERROR;
        goto out;
    }

    /* Check status */
    ret = 0;
    len = sizeof(ret);
    /* char * is for windows... */
    getsockopt(s, SOL_SOCKET, SO_ERROR, (char *)&ret, &len);
    if (ret != 0)
        ret = SOCKET_ERROR;

out:
    poll_disable(s);
    return ret;
}
