
all:
	docker build -t docker/libcontainer .

test:
	# we need NET_ADMIN for the netlink tests and SYS_ADMIN for mounting
	docker run --rm --cap-add NET_ADMIN --cap-add SYS_ADMIN docker/libcontainer

sh:
	docker run --rm -ti -w /busybox --rm --cap-add NET_ADMIN --cap-add SYS_ADMIN docker/libcontainer nsinit exec sh
