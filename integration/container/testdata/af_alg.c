#include <sys/socket.h>
#include <linux/if_alg.h>
#include <unistd.h>
#include <string.h>
#include <stdio.h>

int main() {
    int sockfd, opfd;
    struct sockaddr_alg sa = {
        .salg_family = AF_ALG,
        .salg_type = "hash",
        .salg_name = "sha1"
    };

    sockfd = socket(AF_ALG, SOCK_SEQPACKET, 0);
    if (sockfd < 0) {
        perror("socket");
        return 1;
    }

    if (bind(sockfd, (struct sockaddr *)&sa, sizeof(sa)) < 0) {
        perror("bind");
        close(sockfd);
        return 1;
    }

    opfd = accept(sockfd, NULL, 0);
    if (opfd < 0) {
        perror("accept");
        close(sockfd);
        return 1;
    }

    char data[] = "hello world";
    write(opfd, data, strlen(data));

    char hash[20];
    read(opfd, hash, sizeof(hash));

    printf("SHA1 hash computed\n");

    close(opfd);
    close(sockfd);
    return 0;
}
