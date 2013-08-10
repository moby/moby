#!/bin/sh

# This script looks for bundles built by make.sh, and releases them on a
# public S3 bucket.
#
# Bundles should be available for the VERSION string passed as argument.
#
# The correct way to call this script is inside a container built by the
# official Dockerfile at the root of the Docker source code. The Dockerfile,
# make.sh and release.sh should all be from the same source code revision.

set -x
set -e

# Print a usage message and exit.
usage() {
	echo "Usage: $0 VERSION BUCKET"
	echo "For example: $0 0.5.1-dev sandbox.get.docker.io"
	exit 1
}

VERSION=$1
BUCKET=$2
[ -z "$VERSION" ] && usage
[ -z "$BUCKET" ] && usage

setup_s3() {
	# Try creating the bucket. Ignore errors (it might already exist).
	s3cmd --acl-public mb $BUCKET 2>/dev/null || true
}

# write_to_s3 uploads the contents of standard input to the specified S3 url.
write_to_s3() {
	DEST=$1
	F=`mktemp`
	cat > $F
	s3cmd --acl-public put $F $DEST
	rm -f $F
}

s3_url() {
	echo "http://$BUCKET.s3.amazonaws.com"
}

# Upload the 'ubuntu' bundle to S3:
# 1. A full APT repository is published at $BUCKET/ubuntu/
# 2. Instructions for using the APT repository are uploaded at $BUCKET/ubuntu/info
release_ubuntu() {
	s3cmd --acl-public --verbose --delete-removed --follow-symlinks sync bundles/$VERSION/ubuntu/apt/ s3://$BUCKET/ubuntu/
	cat <<EOF | write_to_s3 s3://$BUCKET/ubuntu/info
# Add the following to /etc/apt/sources.list
deb $(s3_url $BUCKET)/ubuntu docker main
EOF
	echo "APT repository uploaded to http:. Instructions available at $(s3_url $BUCKET)/ubuntu/info"
}

# Upload a static binary to S3
release_binary() {
	[ -e bundles/$VERSION ]
	S3DIR=s3://$BUCKET/builds/Linux/x86_64
	s3cmd --acl-public put bundles/$VERSION/binary/docker-$VERSION $S3DIR/docker-$VERSION
	cat <<EOF | write_to_s3 s3://$BUCKET/builds/info
# To install, run the following command as root:
curl -O http://$BUCKET.s3.amazonaws.com/builds/Linux/x86_64/docker-$VERSION && chmod +x docker-$VERSION && sudo mv docker-$VERSION /usr/local/bin/docker
# Then start docker in daemon mode:
sudo /usr/local/bin/docker -d
EOF
	if [ -z "$NOLATEST" ]; then
		echo "Copying docker-$VERSION to docker-latest"
		s3cmd --acl-public cp $S3DIR/docker-$VERSION $S3DIR/docker-latest
		echo "Advertising $VERSION on $BUCKET as most recent version"
		echo $VERSION | write_to_s3 s3://$BUCKET/latest
	fi
}

main() {
	setup_s3
	release_binary
	release_ubuntu
}

main
