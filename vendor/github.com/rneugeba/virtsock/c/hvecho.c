/*
 * A simple Echo server and client using Hyper-V sockets
 *
 * Works on Linux and Windows (kinda)
 *
 * This was primarily written to checkout shutdown(), which turns out
 * does not work.
 */
#include "compat.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* 3049197C-FACB-11E6-BD58-64006A7986D3 */
DEFINE_GUID(SERVICE_GUID,
    0x3049197c, 0xfacb, 0x11e6, 0xbd, 0x58, 0x64, 0x00, 0x6a, 0x79, 0x86, 0xd3);
#define SERVICE_PORT 0x3049197c

#define MY_BUFLEN 4096

#ifdef _MSC_VER
static WSADATA wsaData;
#endif

/* Use the vsock interface on Linux */
static int opt_vsock;


/* Handle a connection. Echo back anything sent to us and when the
 * connection is closed send a bye message.
 */
static void handle(SOCKET fd)
{
    char recvbuf[MY_BUFLEN];
    int recvbuflen = MY_BUFLEN;
    const char *byebuf = "Bye!";
    int sent;
    int res;

    do {
        res = recv(fd, recvbuf, recvbuflen, 0);
        if (res == 0) {
            printf("Peer closed\n");
            break;
        } else if (res == SOCKET_ERROR) {
            sockerr("recv()");
            return;
        }

        /* No error, echo */
        printf("Bytes received: %d\n", res);
        sent = send(fd, recvbuf, res, 0);
        if (sent == SOCKET_ERROR) {
            sockerr("send()");
            return;
        }
        printf("Bytes sent: %d\n", sent);

    } while (res > 0);

    /* Send bye */
    sent = send(fd, byebuf, sizeof(byebuf), 0);
    if (sent == SOCKET_ERROR) {
        sockerr("send() bye");
        return;
    }
    printf("Bye Bytes sent: %d\n", sent);
}


/* Server:
 * accept() in an endless loop, handle a connection at a time
 */
static int server(void)
{
    SOCKET lsock, csock;
    SOCKADDR_VM savm, sacvm;
    SOCKADDR_HV sahv, sachv;
    socklen_t socklen;
    int res;

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
    savm.SvmPort = SERVICE_PORT;
    savm.SvmCID = VMADDR_CID_ANY; /* Ignore target here */

    memset(&sahv, 0, sizeof(sahv));
    sahv.Family = AF_HYPERV;
    sahv.VmId = HV_GUID_WILDCARD;
    sahv.ServiceId = SERVICE_GUID;

    if (opt_vsock)
        res = bind(lsock, (const struct sockaddr *)&savm, sizeof(savm));
    else
        res = bind(lsock, (const struct sockaddr *)&sahv, sizeof(sahv));
    if (res == SOCKET_ERROR) {
        sockerr("bind()");
        closesocket(lsock);
        return 1;
    }

    res = listen(lsock, SOMAXCONN);
    if (res == SOCKET_ERROR) {
        sockerr("listen()");
        closesocket(lsock);
        return 1;
    }

    while(1) {
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
            closesocket(lsock);
            return 1;
        }

        if (opt_vsock)
            printf("Connect from: 0x%08x.0x%08x\n", sacvm.SvmCID, sacvm.SvmPort);
        else
            printf("Connect from: "GUID_FMT":"GUID_FMT"\n",
                   GUID_ARGS(sachv.VmId), GUID_ARGS(sachv.ServiceId));

        handle(csock);
        closesocket(csock);
    }
}


/* The client sends a messages, and waits for the echo before shutting
 * down the send side. It then expects a bye message from the server.
 */
static int client(GUID target)
{
    SOCKET fd = INVALID_SOCKET;
    SOCKADDR_VM savm;
    SOCKADDR_HV sahv;
    char *sendbuf = "this is a test";
    char recvbuf[MY_BUFLEN];
    int recvbuflen = MY_BUFLEN;
    int res;

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
    savm.SvmPort = SERVICE_PORT;
    savm.SvmCID = VMADDR_CID_ANY; /* Ignore target here */

    memset(&sahv, 0, sizeof(sahv));
    sahv.Family = AF_HYPERV;
    sahv.VmId = target;
    sahv.ServiceId = SERVICE_GUID;

    if (opt_vsock) {
        printf("Connect to: 0x%08x.0x%08x\n", savm.SvmCID, savm.SvmPort);
        res = connect(fd, (const struct sockaddr *)&savm, sizeof(savm));
    } else {
        printf("Connect to: "GUID_FMT":"GUID_FMT"\n",
               GUID_ARGS(sahv.VmId), GUID_ARGS(sahv.ServiceId));
        res = connect(fd, (const struct sockaddr *)&sahv, sizeof(sahv));
    }
    if (res == SOCKET_ERROR) {
        sockerr("connect()");
        goto out;
    }

    res = send(fd, sendbuf, (int)strlen(sendbuf), 0);
    if (res == SOCKET_ERROR) {
        sockerr("send()");
        goto out;
    }

    printf("Bytes Sent: %d\n", res);

    res = recv(fd, recvbuf, recvbuflen, 0);
    if (res < 0) {
        sockerr("recv()");
        goto out;
    } else if (res == 0) {
        printf("Connection closed\n");
        res = 1;
        goto out;
    }

    printf("Bytes received: %d\n", res);
    printf("->%s\n", recvbuf);
    printf("Shutdown\n");

    /* XXX shutdown does not work! */
    res = shutdown(fd, SD_SEND);
    if (res == SOCKET_ERROR) {
        sockerr("shutdown()");
        goto out;
    }

    printf("Wait for bye\n");
    res = recv(fd, recvbuf, recvbuflen, 0);
    if (res < 0) {
        sockerr("recv()");
        goto out;
    } else if (res == 0) {
        printf("Connection closed\n");
        res = 1;
        goto out;
    }

    printf("Bytes received: %d\n", res);
    recvbuf[res] = '\0';
    printf("->%s\n", recvbuf);
    res = 0;

 out:
    closesocket(fd);
    return res;
}

void usage(char *name)
{
    printf("%s: -s | -c <carg> [-vsock]\n", name);
    printf(" -s         Server mode\n");
    printf(" -c <carg>  Client mode. <carg>:\n");
    printf("   'loopback': Connect in loopback mode\n");
    printf("   'parent':   Connect to the parent partition\n");
    printf("   <guid>:     Connect to VM with GUID\n");
    printf(" -vsock     Use AF_VSOCK (Linux only)\n");
}

int __cdecl main(int argc, char **argv)
{
    int opt_server;
    int res = 0;
    GUID target;
    int i;

#ifdef _MSC_VER
    // Initialize Winsock
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

        } else if (strcmp(argv[i], "-vsock") == 0) {
            opt_vsock = 1;
        } else {
            fprintf(stderr, "Unknown argument: %s\n", argv[i]);
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

    if (opt_server)
        res = server();
    else
        res = client(target);

out:
#ifdef _MSC_VER
    WSACleanup();
#endif
    return res;
}
