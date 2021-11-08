// vsockexec opens vsock connections for the specified stdio descriptors and
// then execs the specified process.

#include <errno.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "vsock.h"

#ifdef USE_TCP
static const int tcpmode = 1;
#else
static const int tcpmode;
#endif

static int opentcp(unsigned short port)
{
    int s = socket(AF_INET, SOCK_STREAM, 0);
    if (s < 0) {
        return -1;
    }

    struct sockaddr_in addr = {0};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    if (connect(s, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        return -1;
    }

    return s;
}

_Noreturn static void usage(const char *argv0)
{
    fprintf(stderr, "%s [-i port] [-o port] [-e port] -- program [args...]\n", argv0);
    exit(1);
}

int main(int argc, char **argv)
{
    unsigned int ports[3] = {0};
    int sockets[3] = {-1, -1, -1};
    int c;
    while ((c = getopt(argc, argv, "+i:o:e:")) != -1) {
        switch (c) {
        case 'i':
            ports[0] = strtoul(optarg, NULL, 10);
            break;

        case 'o':
            ports[1] = strtoul(optarg, NULL, 10);
            break;

        case 'e':
            ports[2] = strtoul(optarg, NULL, 10);
            break;

        default:
            usage(argv[0]);
        }
    }

    if (optind == argc) {
        fprintf(stderr, "%s: missing program argument\n", argv[0]);
        usage(argv[0]);
    }

    for (int i = 0; i < 3; i++) {
        if (ports[i] != 0) {
            int j;
            for (j = 0; j < i; j++) {
                if (ports[i] == ports[j]) {
                    int s = dup(sockets[j]);
                    if (s < 0) {
                        perror("dup");
                        return 1;
                    }
                    sockets[i] = s;
                    break;
                }
            }

            if (j == i) {
                int s = tcpmode ? opentcp(ports[i]) : openvsock(VMADDR_CID_HOST, ports[i]);
                if (s < 0) {
                    fprintf(stderr, "connect: port %u: %s", ports[i], strerror(errno));
                    return 1;
                }
                sockets[i] = s;
            }
        }
    }

    for (int i = 0; i < 3; i++) {
        if (sockets[i] >= 0) {
            dup2(sockets[i], i);
            close(sockets[i]);
        }
    }

    execvp(argv[optind], argv + optind);
    fprintf(stderr, "execvp: %s: %s\n", argv[optind], strerror(errno));
    return 1;
}
