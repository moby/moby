# Ubuntu package Makefile
#
# Dependencies:  debhelper autotools-dev devscripts golang-stable
# Notes:
# Use 'make ubuntu' to create the ubuntu package and push it to stating PPA by
# default. To push to production, set PUBLISH_PPA=1 before doing 'make ubuntu'
# GPG_KEY environment variable needs to contain a GPG private key for package
# to be signed and uploaded to docker PPA. If GPG_KEY is not defined,
# make ubuntu will create docker package and exit with status code 2

PKG_NAME=lxc-docker
GITHUB_PATH=github.com/dotcloud/docker
BUILD_SRC=${CURDIR}/../../build_src
VERSION=$(shell sed -En '0,/^\#\# /{s/^\#\# ([^ ]+).+/\1/p}' ../../CHANGELOG.md)

all:
	# Compile docker. Used by dpkg-buildpackage.
	cd src/${GITHUB_PATH}/docker; GOPATH=${CURDIR} CGO_ENABLED=0 go build -a -ldflags '-d -w'

install:
	# Used by dpkg-buildpackage
	mkdir -p ${DESTDIR}/usr/bin
	mkdir -p ${DESTDIR}/etc/init
	mkdir -p ${DESTDIR}/DEBIAN
	install -m 0755 src/${GITHUB_PATH}/docker/docker ${DESTDIR}/usr/bin
	install -o root -m 0644 debian/docker.upstart ${DESTDIR}/etc/init/docker.conf
	install debian/lxc-docker.prerm ${DESTDIR}/DEBIAN/prerm
	install debian/lxc-docker.postinst ${DESTDIR}/DEBIAN/postinst

ubuntu:
	# This Makefile will compile the github master branch of dotcloud/docker
	# Retrieve docker project and its go structure from internet
	rm -rf ${BUILD_SRC}
	git clone $(shell git rev-parse --show-toplevel) ${BUILD_SRC}/${GITHUB_PATH}
	cd ${BUILD_SRC}/${GITHUB_PATH}; git checkout v${VERSION} && GOPATH=${BUILD_SRC} go get -d && cd ${BUILD_SRC}/src/${GITHUB_PATH} && git checkout v${VERSION}
	# Add debianization
	mkdir ${BUILD_SRC}/debian
	cp Makefile ${BUILD_SRC}
	cp -r * ${BUILD_SRC}/debian
	cp ../../README.md ${BUILD_SRC}
	./parse_changelog.py < ../../CHANGELOG.md  > ${BUILD_SRC}/debian/changelog
	# Cleanup
	for d in `find ${BUILD_SRC} -name '.git*'`; do rm -rf $$d; done
	rm -rf ${BUILD_SRC}/../${PKG_NAME}_${VERSION}.orig.tar.gz
	rm -rf ${BUILD_SRC}/pkg
	# Create docker debian files
	cd ${BUILD_SRC}; tar czf ../${PKG_NAME}_${VERSION}.orig.tar.gz .
	cd ${BUILD_SRC}; dpkg-buildpackage -us -uc
	rm -rf ${BUILD_SRC}
	# Sign package and upload it to PPA if GPG_KEY environment variable
	# holds a private GPG KEY
	if /usr/bin/test "$${GPG_KEY}" == ""; then exit 2; fi
	mkdir ${BUILD_SRC}
	# Import gpg signing key
	echo "$${GPG_KEY}" | gpg --allow-secret-key-import --import || true
	# Sign the package
	cd ${BUILD_SRC}; dpkg-source -x ${BUILD_SRC}/../${PKG_NAME}_${VERSION}-1.dsc
	cd ${BUILD_SRC}/${PKG_NAME}-${VERSION}; debuild -S -sa
	# Upload to PPA
	if [ "${PUBLISH_PPA}" = "1" ];  then cd ${BUILD_SRC};dput ppa:dotcloud/lxc-docker ${PKG_NAME}_${VERSION}-1_source.changes; fi
	if [ "${PUBLISH_PPA}" != "1" ]; then cd ${BUILD_SRC};dput ppa:dotcloud/docker-staging ${PKG_NAME}_${VERSION}-1_source.changes; fi
	rm -rf ${BUILD_SRC}
