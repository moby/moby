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

# We're a nice, sexy, little shell script, and people might try to run us;
# but really, they shouldn't. We want to be in a container!
RESOLVCONF=$(readlink --canonicalize /etc/resolv.conf)
grep -q "$RESOLVCONF" /proc/mounts || {
	echo "# I will only run within a container."
	echo "# Try this instead:"
	echo "docker build ."
	exit 1
}


SCRIPT_DIR=`dirname "$0"`
source $SCRIPT_DIR/common.sh

PACKAGE_ARCHITECTURE="$(dpkg-architecture -qDEB_HOST_ARCH)"
PACKAGE_URL="http://www.docker.io/"
PACKAGE_MAINTAINER="docker@dotcloud.com"
PACKAGE_DESCRIPTION="lxc-docker is a Linux container runtime
Docker complements LXC with a high-level API which operates at the process
level. It runs unix processes with strong guarantees of isolation and
repeatability across servers.
Docker is a great building block for automating distributed systems:
large-scale web deployments, database clusters, continuous deployment systems,
private PaaS, service-oriented architectures, etc."

UPSTART_SCRIPT='description     "Docker daemon"

start on filesystem or runlevel [2345]
stop on runlevel [!2345]

respawn

script
    /usr/bin/docker -d
end script
'

# Each "bundle" is a different type of build artefact: static binary, Ubuntu
# package, etc.

# Build Docker as a static binary file
bundle_binary() {
	mkdir -p bundles/$VERSION/binary
	go build -o bundles/$VERSION/binary/docker-$VERSION \
		-ldflags "$LDFLAGS" ./docker
}


# Build Docker's test suite as a collection of binary files (one per
# sub-package to test)
bundle_test() {
	mkdir -p bundles/$VERSION/test
	for test_dir in $(find_test_dirs); do
		test_binary=$(
			cd $test_dir
			go test -c -v -ldflags "-X main.GITCOMMIT $GITCOMMIT -X main.VERSION $VERSION -d -w" >&2
			find . -maxdepth 1 -type f -name '*.test' -executable
		)
		cp $test_dir/$test_binary bundles/$VERSION/test/
	done
}

# Build docker as an ubuntu package using FPM and REPREPRO (sue me).
# bundle_binary must be called first.
bundle_ubuntu() {
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
	bundle_binary
	bundle_ubuntu
	#bundle_test
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

main
