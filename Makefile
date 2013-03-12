BUILD_PATH:=$(shell pwd)/build
INSDIR=/opt/docker
ifdef DESTDIR
        INSDIR=usr/bin
endif

all:
	GOPATH=$(BUILD_PATH) go get github.com/dotcloud/docker
	cd build/src/github.com/dotcloud/docker/docker; GOPATH=$(BUILD_PATH) go build
	cd build/src/github.com/dotcloud/docker/dockerd; GOPATH=$(BUILD_PATH) go build

install: all
	mkdir -p $(DESTDIR)/$(INSDIR)
	mkdir -p $(DESTDIR)/etc/init
	install -m 0755 $(BUILD_PATH)/src/github.com/dotcloud/docker/docker/docker $(DESTDIR)/$(INSDIR)
	install -o root -m 0755 $(BUILD_PATH)/src/github.com/dotcloud/docker/dockerd/dockerd $(DESTDIR)/$(INSDIR)
	install -o root -m 0755 $(BUILD_PATH)/../debian/dockerd.upstart $(DESTDIR)/etc/init/dockerd.conf

clean:
	rm -rf build debian/dockerd
