#!/bin/sh

# This script builds various binary artifacts from a checkout of the docker
# source code without docker installed first. It should be used for distro
# packaging only.

set -e

. hack/release/common.sh

prepare_gopath() {
	DOCKER_VENDOR=$PWD/vendor/src/github.com/dotcloud/docker
	if [ -h $DOCKER_VENDOR ]; then
		rm -f $DOCKER_VENDOR;
	fi

	ln -sf $PWD $DOCKER_VENDOR
}

main() {
        cat <<EOF
###############################################################################

 This version of the build is unsupported. It is your responsibility to ensure
 all dependencies are met and that the right version of go is used.

###############################################################################
EOF
	prepare_gopath
	BIN_TARGET=bin/docker
	GOPATH=$PWD/vendor bundle_binary $BIN_TARGET
	echo $BIN_TARGET is created.
}

main
