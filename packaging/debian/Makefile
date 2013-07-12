# Debian package Makefile
#
# Dependencies:  git debhelper build-essential autotools-dev devscripts golang
# Notes:
# Use 'make debian' to create the debian package
# To create a specific version, use 'VERSION_TAG=v0.2.0 make debian'
# GPG_KEY environment variable needs to contain a GPG private key for package
# to be signed and uploaded to debian.
# If GPG_KEY is not defined, make debian will create docker package and exit
# with status code 2

PKG_NAME=lxc-docker
ROOT_PATH=$(shell git rev-parse --show-toplevel)
GITHUB_PATH=github.com/dotcloud/docker
BUILD_SRC=build_src
VERSION=$(shell sed -En '0,/^\#\# /{s/^\#\# ([^ ]+).+/\1/p}' ../../CHANGELOG.md)
VERSION_TAG?=v${VERSION}
DOCKER_VERSION=${PKG_NAME}_${VERSION}

all:
	# Compile docker. Used by debian dpkg-buildpackage.
	cd src/${GITHUB_PATH}/docker; GOPATH=${CURDIR} go build

install:
	# Used by debian dpkg-buildpackage
	mkdir -p $(DESTDIR)/usr/bin
	mkdir -p $(DESTDIR)/usr/share/man/man1
	mkdir -p $(DESTDIR)/usr/share/doc/lxc-docker
	install -m 0755 src/${GITHUB_PATH}/docker/docker $(DESTDIR)/usr/bin/lxc-docker
	cp debian/lxc-docker.1 $(DESTDIR)/usr/share/man/man1

debian:
	# Prepare docker source from revision ${VERSION_TAG}
	rm -rf ${BUILD_SRC} ${PKG_NAME}_[0-9]*
	git clone file://$(ROOT_PATH) ${BUILD_SRC}/src/${GITHUB_PATH} --branch ${VERSION_TAG} --depth 1
	GOPATH=${CURDIR}/${BUILD_SRC} go get -d ${GITHUB_PATH}
	# Add debianization
	mkdir ${BUILD_SRC}/debian
	cp Makefile ${BUILD_SRC}
	cp -r `ls | grep -v ${BUILD_SRC}` ${BUILD_SRC}/debian
	cp ${ROOT_PATH}/README.md ${BUILD_SRC}
	cp ${ROOT_PATH}/CHANGELOG.md ${BUILD_SRC}/debian
	./parse_changelog.py < ../../CHANGELOG.md  > ${BUILD_SRC}/debian/changelog
	# Cleanup
	rm -rf `find . -name '.git*'`
	rm -f ${DOCKER_VERSION}*
	# Create docker debian files
	cd ${BUILD_SRC}; tar czf ../${DOCKER_VERSION}.orig.tar.gz .
	cd ${BUILD_SRC}; dpkg-buildpackage -us -uc
	rm -rf ${BUILD_SRC}
	# Sign package and upload it to PPA if GPG_KEY environment variable
	# holds a private GPG KEY
	if /usr/bin/test "$${GPG_KEY}" == ""; then exit 2; fi
	mkdir ${BUILD_SRC}
	# Import gpg signing key
	echo "$${GPG_KEY}" | gpg --allow-secret-key-import --import
	# Sign the package
	cd ${BUILD_SRC}; dpkg-source -x ${CURDIR}/${DOCKER_VERSION}-1.dsc
	cd ${BUILD_SRC}/${PKG_NAME}-${VERSION}; debuild -S -sa
