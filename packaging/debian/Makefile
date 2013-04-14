PKG_NAME=lxc-docker
DOCKER_VERSION=$(shell head -1 changelog | awk 'match($$0, /\(.+\)/) {print substr($$0, RSTART+1, RLENGTH-4)}')
GITHUB_PATH=github.com/dotcloud/docker
SOURCE_PKG=$(PKG_NAME)_$(DOCKER_VERSION).orig.tar.gz
BUILD_SRC=${CURDIR}/../../build_src

all:
	# Compile docker. Used by debian dpkg-buildpackage. 
	cd src/${GITHUB_PATH}/docker; GOPATH=${CURDIR} go build

install:
	# Used by debian dpkg-buildpackage 
	mkdir -p $(DESTDIR)/usr/bin
	mkdir -p $(DESTDIR)/etc/init.d
	install -m 0755 src/${GITHUB_PATH}/docker/docker $(DESTDIR)/usr/bin
	install -o root -m 0755 debian/docker.initd $(DESTDIR)/etc/init.d/docker

debian:
	# This Makefile will compile the github master branch of dotcloud/docker
	# Retrieve docker project and its go structure from internet
	rm -rf ${BUILD_SRC}
	GOPATH=${BUILD_SRC} go get ${GITHUB_PATH}
	# Add debianization
	mkdir ${BUILD_SRC}/debian
	cp Makefile ${BUILD_SRC}
	cp -r * ${BUILD_SRC}/debian
	cp ../../README.md ${BUILD_SRC}
	# Cleanup
	for d in `find ${BUILD_SRC} -name '.git*'`; do rm -rf $$d; done
	rm -rf ${BUILD_SRC}/../${SOURCE_PKG}
	rm -rf ${BUILD_SRC}/pkg
	# Create docker debian files
	cd ${BUILD_SRC}; tar czf ../${SOURCE_PKG} .
	cd ${BUILD_SRC}; dpkg-buildpackage
	rm -rf ${BUILD_SRC}
