# common build utilities and variables used in make.sh and
# make-without-docker.sh

VERSION=$(cat ./VERSION)
PKGVERSION="$VERSION"
GITCOMMIT=$(git rev-parse --short HEAD)
if test -n "$(git status --porcelain)"
then
	GITCOMMIT="$GITCOMMIT-dirty"
	PKGVERSION="$PKGVERSION-$(date +%Y%m%d%H%M%S)-$GITCOMMIT"
fi

PACKAGE_URL="http://www.docker.io/"
PACKAGE_MAINTAINER="docker@dotcloud.com"
PACKAGE_DESCRIPTION="lxc-docker is a Linux container runtime
Docker complements LXC with a high-level API which operates at the process
level. It runs unix processes with strong guarantees of isolation and
repeatability across servers.
Docker is a great building block for automating distributed systems:
large-scale web deployments, database clusters, continuous deployment systems,
private PaaS, service-oriented architectures, etc."

LDFLAGS="-X main.GITCOMMIT $GITCOMMIT -X main.VERSION $VERSION -w"

export CGO_ENABLED="0"

# Each "bundle" is a different type of build artefact: static binary, Ubuntu
# package, etc.

# Build Docker as a static binary file
bundle_binary() {
	TARGET=bundles/$VERSION/binary/docker-$VERSION

	if [ $# -eq 1 ]; then
		TARGET=$1
	fi

	mkdir -p $(dirname $TARGET)

	go build -o $TARGET -ldflags "$LDFLAGS" ./docker
}
