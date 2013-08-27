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
#   with the suffix -dirty if the repository isn't clean.
# - The script is intented to be run as part of a docker build, as defined
#   in the Dockerfile at the root of the source. In other words:
#   DO NOT CALL THIS SCRIPT DIRECTLY.
# - The right way to call this script is to invoke "docker build ." from
#   your checkout of the Docker repository.
# 

set -e

VERSION=$(cat ./VERSION)
PKGVERSION="$VERSION"
GITCOMMIT=$(git rev-parse --short HEAD)
if test -n "$(git status --porcelain)"
then
	GITCOMMIT="$GITCOMMIT-dirty"
	PKGVERSION="$PKGVERSION-$(date +%Y%m%d%H%M%S)-$GITCOMMIT"
fi

LDFLAGS="-X main.GITCOMMIT $GITCOMMIT -X main.VERSION $VERSION -w"

export CGO_ENABLED="0"

prepare_gopath() {
	DOCKER_VENDOR=$PWD/vendor/src/github.com/dotcloud/docker
	if [ -h $DOCKER_VENDOR ]; then
		rm -f $DOCKER_VENDOR;
	fi

	ln -sf $PWD $DOCKER_VENDOR
}

# Each "bundle" is a different type of build artefact: static binary, Ubuntu
# package, etc.

# Build Docker as a static binary file
bundle_binary() {
	TARGET=bundles/$VERSION/binary/docker-$VERSION

	# Unofficial builds will pass in a different target
	if [ $# -eq 1 ]; then
		TARGET=$1
	else
		# TODO: For some reason binary versions of golang from golang.org don't like
		# this flag. Add it into the official build as it had it originally and it
		# seems to work. This keeps developers who use make-without-docker.sh from
		# being confused by default.

		# Setup LDFLAGS for the official build
		LDFLAGS="$LDFLAGS -d"
	fi

	mkdir -p $(dirname $TARGET)

	go build -o $TARGET -ldflags "$LDFLAGS" ./docker
}

# Build an unofficial static binary file
bundle_unofficial_binary() {
        cat <<EOF
###############################################################################

 This version of the build is unsupported. It is your responsibility to ensure
 all dependencies are met and that the right version of go is used.

###############################################################################
EOF
	prepare_gopath
	BIN_TARGET=bin/docker
	GOPATH="$PWD/vendor" bundle_binary $BIN_TARGET
	echo $BIN_TARGET is created.
}

# Build Docker's test suite as a collection of binary files (one per
# sub-package to test)
bundle_test() {
	mkdir -p bundles/$VERSION/test
	for test_dir in $(find_test_dirs); do
		test_binary=$(
			cd $test_dir
			go test -c -v -ldflags "$LDFLAGS" >&2
			find . -maxdepth 1 -type f -name '*.test' -executable
		)
		cp $test_dir/$test_binary bundles/$VERSION/test/
	done
}

# Build docker as an ubuntu package using FPM and REPREPRO (sue me).
# bundle_binary must be called first.
bundle_ubuntu() {
	. hack/release/ubuntu.sh
	mkdir -p bundles/$VERSION/ubuntu

	DIR=$(pwd)/bundles/$VERSION/ubuntu/build

	# Generate an upstart config file (ubuntu-specific)
	mkdir -p $DIR/etc/init
	echo "$UPSTART_SCRIPT" > $DIR/etc/init/docker.conf

	# Copy the binary
	mkdir -p $DIR/usr/bin
	cp bundles/$VERSION/binary/docker-$VERSION $DIR/usr/bin/docker

	# Generate postinstall/prerm scripts
	cat >/tmp/postinstall <<EOF
#!/bin/sh
/sbin/stop docker || true
/sbin/start docker
EOF
	cat >/tmp/prerm <<EOF
#!/bin/sh
/sbin/stop docker || true
EOF
	chmod +x /tmp/postinstall /tmp/prerm

	(
		cd bundles/$VERSION/ubuntu
		fpm -s dir -C $DIR \
		    --name lxc-docker-$VERSION --version $PKGVERSION \
		    --after-install /tmp/postinstall \
		    --before-remove /tmp/prerm \
		    --architecture "$PACKAGE_ARCHITECTURE" \
		    --prefix / \
		    --depends lxc --depends aufs-tools \
		    --description "$PACKAGE_DESCRIPTION" \
		    --maintainer "$PACKAGE_MAINTAINER" \
		    --conflicts lxc-docker-virtual-package \
		    --provides lxc-docker \
		    --provides lxc-docker-virtual-package \
		    --replaces lxc-docker \
		    --replaces lxc-docker-virtual-package \
		    --url "$PACKAGE_URL" \
		    --vendor "$PACKAGE_VENDOR" \
		    -t deb .
		mkdir empty
		fpm -s dir -C empty \
		    --name lxc-docker --version $PKGVERSION \
		    --architecture "$PACKAGE_ARCHITECTURE" \
		    --depends lxc-docker-$VERSION \
		    --description "$PACKAGE_DESCRIPTION" \
		    --maintainer "$PACKAGE_MAINTAINER" \
		    --url "$PACKAGE_URL" \
		    --vendor "$PACKAGE_VENDOR" \
		    -t deb .
	)
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
	TARGETS=unofficial_binary

	if [ $# -ne 0 ]; then
		TARGETS=$@
	fi

	prepare_gopath
	for i in $TARGETS; do
		bundle_$i
	done

	if [ "$TARGETS" = "unofficial_binary" ]; then
		return
	fi

	cat <<EOF
###############################################################################
Now run the resulting image, making sure that you set AWS_S3_BUCKET,
AWS_ACCESS_KEY, and AWS_SECRET_KEY environment variables:

docker run -e AWS_S3_BUCKET=get-staging.docker.io \\
              AWS_ACCESS_KEY=AKI1234... \\
              AWS_SECRET_KEY=sEs3mE... \\
              GPG_PASSPHRASE=sesame... \\
              image_id_or_name
###############################################################################
EOF
}

main $@
