#!/usr/bin/env bash
set -e

set -o pipefail

usage() {
	cat >&2 <<'EOF'
To publish the Docker documentation you need to set your access_key and secret_key in the docs/s3cfg file
and set the AWS_S3_BUCKET env var to the name of your bucket.

make AWS_S3_BUCKET=beta-docs.docker.io docs-release

will then push the documentation site to your s3 bucket.
EOF
	exit 1
}

[ "$AWS_S3_BUCKET" ] || usage

#VERSION=$(cat VERSION)
BUCKET=$AWS_S3_BUCKET

[ -e s3cfg ] || usage
cp s3cfg ${HOME}/.s3cfg

setup_s3() {
	# Try creating the bucket. Ignore errors (it might already exist).
	s3cmd mb s3://$BUCKET 2>/dev/null || true
	# Check access to the bucket.
	# s3cmd has no useful exit status, so we cannot check that.
	# Instead, we check if it outputs anything on standard output.
	# (When there are problems, it uses standard error instead.)
	s3cmd info s3://$BUCKET | grep -q .
	# Make the bucket accessible through website endpoints.
	s3cmd ws-create --ws-index index.html --ws-error error.html s3://$BUCKET
}

build_current_documentation() {
	mkdocs build
}

upload_current_documentation() {
	src=$(pwd)/site/
	dst=s3://$BUCKET

	echo
	echo "Uploading $src"
	echo "  to $dst"
	echo
	s3cmd --recursive --follow-symlinks --preserve --acl-public sync "$src" "$dst"
}

setup_s3
build_current_documentation
upload_current_documentation

