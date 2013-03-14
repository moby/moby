PKG_NAME=docker-dev
PKG_VERSION=1
ROOT_PATH:=$(PWD)
BUILD_PATH=build
BUILD_SRC=build_src
GITHUB_PATH=src/github.com/dotcloud/docker
INSDIR=usr/bin

all:
	cp -r $(BUILD_SRC) $(BUILD_PATH)
	cd $(BUILD_PATH)/$(GITHUB_PATH)/docker; GOPATH=$(ROOT_PATH)/$(BUILD_PATH) go build

# DESTDIR provided by Debian packaging
install: all
	mkdir -p $(DESTDIR)/$(INSDIR)
	mkdir -p $(DESTDIR)/etc/init
	install -m 0755 $(BUILD_PATH)/$(GITHUB_PATH)/docker/docker $(DESTDIR)/$(INSDIR)
	install -o root -m 0755 $(ROOT_PATH)/etc/docker-dev.upstart $(DESTDIR)/etc/init/docker-dev.conf

# Build deb package fetching go dependencies and cleaning up git repositories
deb: cleanup
	GOPATH=$(ROOT_PATH)/$(BUILD_SRC) go get -d github.com/dotcloud/docker
	for d in `find . -name '.git*'`; do rm -rf $$d; done
	tar czf ../$(PKG_NAME)_$(PKG_VERSION).orig.tar.gz *
	dpkg-buildpackage
	rm -rf $(BUILD_PATH) debian/$(PKG_NAME)* debian/files

cleanup:
	rm -rf $(BUILD_PATH) debian/$(PKG_NAME)* debian/files $(BUILD_SRC)
