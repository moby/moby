# This file describes the standard way to build Docker, using docker
#
# Usage:
#
# # Assemble the full dev environment. This is slow the first time.
# docker build -t docker .
#
# # Mount your source in an interactive container for quick testing:
# docker run -v `pwd`:/go/src/github.com/dotcloud/docker -privileged -i -t docker bash
#
# # Run the test suite:
# docker run -privileged docker hack/make.sh test
#
# # Publish a release:
# docker run -privileged \
#  -e AWS_S3_BUCKET=baz \
#  -e AWS_ACCESS_KEY=foo \
#  -e AWS_SECRET_KEY=bar \
#  -e GPG_PASSPHRASE=gloubiboulga \
#  docker hack/release.sh
#
# Note: Apparmor used to mess with privileged mode, but this is no longer
# the case. Therefore, you don't have to disable it anymore.
#

docker-version	0.6.1
FROM	ubuntu:12.04
MAINTAINER	Solomon Hykes <solomon@dotcloud.com>

# Build dependencies
RUN	echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
RUN	apt-get update
RUN	apt-get install -y -q curl
RUN	apt-get install -y -q git
RUN	apt-get install -y -q mercurial
RUN	apt-get install -y -q build-essential libsqlite3-dev

# Install Go
RUN	curl -s https://go.googlecode.com/files/go1.2.src.tar.gz | tar -v -C /usr/local -xz
ENV	PATH	/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
ENV	GOPATH	/go:/go/src/github.com/dotcloud/docker/vendor
RUN	cd /usr/local/go/src && ./make.bash && go install -ldflags '-w -linkmode external -extldflags "-static -Wl,--unresolved-symbols=ignore-in-shared-libs"' -tags netgo -a std

# Ubuntu stuff
RUN	apt-get install -y -q ruby1.9.3 rubygems libffi-dev
RUN	gem install --no-rdoc --no-ri fpm
RUN	apt-get install -y -q reprepro dpkg-sig

RUN	apt-get install -y -q python-pip
RUN	pip install s3cmd==1.1.0-beta3
RUN	pip install python-magic==0.4.6
RUN	/bin/echo -e '[default]\naccess_key=$AWS_ACCESS_KEY\nsecret_key=$AWS_SECRET_KEY\n' > /.s3cfg

# Runtime dependencies
RUN	apt-get install -y -q iptables
RUN	apt-get install -y -q lxc
RUN	apt-get install -y -q aufs-tools

# Get lvm2 source for compiling statically
RUN	git clone https://git.fedorahosted.org/git/lvm2.git /usr/local/lvm2 && cd /usr/local/lvm2 && git checkout v2_02_103
# see https://git.fedorahosted.org/cgit/lvm2.git/refs/tags for release tags
# note: we can't use "git clone -b" above because it requires at least git 1.7.10 to be able to use that on a tag instead of a branch and we only have 1.7.9.5

# Compile and install lvm2
RUN	cd /usr/local/lvm2 && ./configure --enable-static_link && make device-mapper && make install_device-mapper
# see https://git.fedorahosted.org/cgit/lvm2.git/tree/INSTALL

VOLUME	/var/lib/docker
WORKDIR	/go/src/github.com/dotcloud/docker

# Wrap all commands in the "docker-in-docker" script to allow nested containers
ENTRYPOINT	["hack/dind"]

# Upload docker source
ADD	.	/go/src/github.com/dotcloud/docker
