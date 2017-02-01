#include <stdio.h>
#include <sys/socket.h>

int main() {

	if (socket(AF_APPLETALK, SOCK_DGRAM, 0) != -1) {
		fprintf(stderr, "Opening Appletalk socket worked, should be blocked\n");
		return 1;
	}

	return 0;
}
