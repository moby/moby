#!/usr/bin/env bash
set -e

set -o pipefail

usage() {
	cat >&2 <<'EOF'
To publish the Docker documentation you need to set your access_key and secret_key in the docs/awsconfig file 
(with the keys in a [profile $AWS_S3_BUCKET] section - so you can have more than one set of keys in your file)
and set the AWS_S3_BUCKET env var to the name of your bucket.

make AWS_S3_BUCKET=beta-docs.docker.io docs-release

will then push the documentation site to your s3 bucket.
EOF
	exit 1
}

[ "$AWS_S3_BUCKET" ] || usage

#VERSION=$(cat VERSION)
BUCKET=$AWS_S3_BUCKET

export AWS_CONFIG_FILE=$(pwd)/awsconfig
[ -e "$AWS_CONFIG_FILE" ] || usage
export AWS_DEFAULT_PROFILE=$BUCKET

echo "cfg file: $AWS_CONFIG_FILE ; profile: $AWS_DEFAULT_PROFILE"

setup_s3() {
	echo "Create $BUCKET"
	# Try creating the bucket. Ignore errors (it might already exist).
	aws s3 mb s3://$BUCKET 2>/dev/null || true
	# Check access to the bucket.
	echo "test $BUCKET exists"
	aws s3 ls s3://$BUCKET
	# Make the bucket accessible through website endpoints.
	echo "make $BUCKET accessible as a website"
	#aws s3 website s3://$BUCKET --index-document index.html --error-document jsearch/index.html
	s3conf=$(cat s3_website.json)
	aws s3api put-bucket-website --bucket $BUCKET --website-configuration "$s3conf"
}

build_current_documentation() {
	mkdocs build
}

upload_current_documentation() {
	src=site/
	dst=s3://$BUCKET

	echo
	echo "Uploading $src"
	echo "  to $dst"
	echo
	#s3cmd --recursive --follow-symlinks --preserve --acl-public sync "$src" "$dst"
	aws s3 sync --acl public-read --exclude "*.rej" --exclude "*.rst" --exclude "*.orig" --exclude "*.py" "$src" "$dst"
}

setup_s3
build_current_documentation
upload_current_documentation

