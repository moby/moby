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
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>

# Build dependencies
run	echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
run	apt-get update
run	apt-get install -y -q curl
run	apt-get install -y -q git
run	apt-get install -y -q mercurial
run	apt-get install -y -q build-essential libsqlite3-dev

# Install Go
run	curl -s https://go.googlecode.com/files/go1.2.src.tar.gz | tar -v -C /usr/local -xz
env	PATH	/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
env	GOPATH	/go:/go/src/github.com/dotcloud/docker/vendor
run	cd /usr/local/go/src && ./make.bash && go install -ldflags '-w -linkmode external -extldflags "-static -Wl,--unresolved-symbols=ignore-in-shared-libs"' -tags netgo -a std

# Ubuntu stuff
run	apt-get install -y -q ruby1.9.3 rubygems libffi-dev
run	gem install --no-rdoc --no-ri fpm
run	apt-get install -y -q reprepro dpkg-sig

run	apt-get install -y -q python-pip
run	pip install s3cmd==1.1.0-beta3
run	pip install python-magic==0.4.6
run	/bin/echo -e '[default]\naccess_key=$AWS_ACCESS_KEY\nsecret_key=$AWS_SECRET_KEY\n' > /.s3cfg

# Runtime dependencies
run	apt-get install -y -q iptables
run	apt-get install -y -q lxc
run	apt-get install -y -q aufs-tools

# Get lvm2 source for compiling statically
run	git clone https://git.fedorahosted.org/git/lvm2.git /usr/local/lvm2 && cd /usr/local/lvm2 && git checkout v2_02_103
# see https://git.fedorahosted.org/cgit/lvm2.git/refs/tags for release tags
# note: we can't use "git clone -b" above because it requires at least git 1.7.10 to be able to use that on a tag instead of a branch and we only have 1.7.9.5

# Compile and install lvm2
run	cd /usr/local/lvm2 && ./configure --enable-static_link && make device-mapper && make install_device-mapper
# see https://git.fedorahosted.org/cgit/lvm2.git/tree/INSTALL

volume	/var/lib/docker
workdir	/go/src/github.com/dotcloud/docker

# Wrap all commands in the "docker-in-docker" script to allow nested containers
entrypoint	["hack/dind"]

# Upload docker source
add	.	/go/src/github.com/dotcloud/docker
