#include <stdio.h>
#include <errno.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/mman.h>

#define SYS_SOCKETCALL_I386 102
#define SYS_SOCKET 1

#ifndef SOCK_FAMILY
#error "define SOCK_FAMILY via -DSOCK_FAMILY=..."
#endif
#ifndef SOCK_TYPE
#error "define SOCK_TYPE via -DSOCK_TYPE=..."
#endif

int main() {
    /*
     * The int $0x80 ia32 compat path truncates all registers to 32 bits.
     * The args pointer must live below 4 GB, so allocate it with MAP_32BIT.
     */
    unsigned int *args = mmap(NULL, 4096,
        PROT_READ | PROT_WRITE,
        MAP_PRIVATE | MAP_ANONYMOUS | MAP_32BIT,
        -1, 0);
    if (args == MAP_FAILED) {
        perror("mmap");
        return 2;
    }
    args[0] = SOCK_FAMILY;
    args[1] = SOCK_TYPE;
    args[2] = 0;

    int ret;
    asm volatile (
        "int $0x80"
        : "=a"(ret)
        : "a"(SYS_SOCKETCALL_I386), "b"(SYS_SOCKET), "c"(args)
        : "memory"
    );

    if (ret < 0) {
        errno = -ret;
        perror("socket");
        return 1;
    }

    printf("socket(%d, %d, 0) via socketcall succeeded\n", SOCK_FAMILY, SOCK_TYPE);
    close(ret);
    return 0;
}
