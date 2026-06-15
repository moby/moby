#include <stdio.h>
#include <errno.h>
#include <sys/mman.h>

#define SYS_SOCKETCALL_I386 102
#define SYS_SOCKET 1
#define AF_ALG 38
#define SOCK_SEQPACKET 5

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
    args[0] = AF_ALG;
    args[1] = SOCK_SEQPACKET;
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

    printf("AF_ALG socket created via socketcall\n");
    return 0;
}
