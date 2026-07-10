#include <sys/socket.h>
#include <linux/vm_sockets.h>
#include <unistd.h>
#include <stdio.h>

int main() {
    int sockfd = socket(AF_VSOCK, SOCK_STREAM, 0);
    if (sockfd < 0) {
        perror("socket");
        return 1;
    }

    printf("AF_VSOCK socket created\n");
    close(sockfd);
    return 0;
}
