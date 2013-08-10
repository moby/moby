#!/bin/sh

# This script builds various binary artifacts from a checkout of the docker
# source code.
#
# Requirements:
# - The current directory should be a checkout of the docker source code
#   (http://github.com/dotcloud/docker). Whatever version is checked out
#   will be built.
# - The VERSION file, at the root of the repository, should exist, and
#   will be used as Docker binary version and package version.
# - The hash of the git commit will also be included in the Docker binary,
#   with the suffix +CHANGES if the repository isn't clean.
# - The script is intented to be run as part of a docker build, as defined
#   in the Dockerfile at the root of the source. In other words:
#   DO NOT CALL THIS SCRIPT DIRECTLY.
# - The right way to call this script is to invoke "docker build ." from
#   your checkout of the Docker repository.
# 

set -e
set -x

VERSION=$(cat ./VERSION)
GIT_COMMIT=$(git rev-parse --short HEAD)
GIT_CHANGES=$(test -n "$(git status --porcelain)" && echo "+CHANGES" || true)

# Each "bundle" is a different type of build artefact: static binary, Ubuntu
# package, etc.

# Build Docker as a static binary file
bundle_binary() {
	mkdir -p bundles/$VERSION/binary
	go build -o bundles/$VERSION/binary/docker-$VERSION \
		-ldflags "-X main.GITCOMMIT $GIT_COMMIT$GIT_CHANGES -X main.VERSION $VERSION -d -w" \
		./docker
}


# Build Docker's test suite as a collection of binary files (one per
# sub-package to test)
bundle_test() {
	mkdir -p bundles/$VERSION/test
	for test_dir in $(find_test_dirs); do
		test_binary=$(
			cd $test_dir
			go test -c -v -ldflags "-X main.GITCOMMIT $GIT_COMMIT$GIT_CHANGES -X main.VERSION $VERSION -d -w" >&2
			find . -maxdepth 1 -type f -name '*.test' -executable
		)
		cp $test_dir/$test_binary bundles/$VERSION/test/
	done
}

# Build docker as an ubuntu package using FPM and REPREPRO (sue me).
# bundle_binary must be called first.
bundle_ubuntu() {
	mkdir -p bundles/$VERSION/ubuntu

	DIR=$(mktemp -d)

	# Generate an upstart config file (ubuntu-specific)
	mkdir -p $DIR/etc/init
	cat > $DIR/etc/init/docker.conf <<EOF
description     "Run docker"

start on filesystem or runlevel [2345]
stop on runlevel [!2345]

respawn

exec docker -d
EOF

	# Copy the binary
	mkdir -p $DIR/usr/bin
	cp bundles/$VERSION/binary/docker-$VERSION $DIR/usr/bin/docker

	(
		cd bundles/$VERSION/ubuntu
		fpm -s dir -t deb -n lxc-docker -v $VERSION -a all --prefix / -C $DIR .
	)
	rm -fr $DIR


	# Setup the APT repo
	APTDIR=bundles/$VERSION/ubuntu/apt
	mkdir -p $APTDIR/conf
	cat > $APTDIR/conf/distributions <<EOF
Codename: docker
Components: main
Architectures: amd64
EOF

	# Add the DEB package to the APT repo
	DEBFILE=bundles/$VERSION/ubuntu/lxc-docker*.deb
	reprepro -b $APTDIR includedeb docker $DEBFILE
}


# This helper function walks the current directory looking for directories
# holding Go test files, and prints their paths on standard output, one per
# line.
find_test_dirs() {
	find . -name '*_test.go' | 
		{ while read f; do dirname $f; done; } | 
		sort -u
}


main() {
	bundle_binary
	bundle_ubuntu
	#bundle_test
}

main
