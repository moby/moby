#!/usr/bin/env bash
set -e

set -o pipefail

usage() {
	cat >&2 <<'EOF'
To publish the Docker documentation you need to set your access_key and secret_key in the docs/awsconfig file 
(with the keys in a [profile $AWS_S3_BUCKET] section - so you can have more than one set of keys in your file)
and set the AWS_S3_BUCKET env var to the name of your bucket.

If you're publishing the current release's documentation, also set `BUILD_ROOT=yes`

make AWS_S3_BUCKET=docs-stage.docker.com docs-release

will then push the documentation site to your s3 bucket.

 Note: you can add `OPTIONS=--dryrun` to see what will be done without sending to the server
EOF
	exit 1
}

[ "$AWS_S3_BUCKET" ] || usage

VERSION=$(cat VERSION)

if [ "$AWS_S3_BUCKET" == "docs.docker.com" ]; then
	if [ "${VERSION%-dev}" != "$VERSION" ]; then
		echo "Please do not push '-dev' documentation to docs.docker.com ($VERSION)"
		exit 1
	fi
	cat > ./sources/robots.txt <<'EOF'
User-agent: *
Allow: /
EOF

else
	cat > ./sources/robots.txt <<'EOF'
User-agent: *
Disallow: /
EOF
fi

# Remove the last version - 1.0.2-dev -> 1.0
MAJOR_MINOR="v${VERSION%.*}"
export MAJOR_MINOR

export BUCKET=$AWS_S3_BUCKET

export AWS_CONFIG_FILE=$(pwd)/awsconfig
[ -e "$AWS_CONFIG_FILE" ] || usage
export AWS_DEFAULT_PROFILE=$BUCKET

echo "cfg file: $AWS_CONFIG_FILE ; profile: $AWS_DEFAULT_PROFILE"

setup_s3() {
	echo "Create $BUCKET"
	# Try creating the bucket. Ignore errors (it might already exist).
	aws s3 mb --profile $BUCKET s3://$BUCKET 2>/dev/null || true
	# Check access to the bucket.
	echo "test $BUCKET exists"
	aws s3 --profile $BUCKET ls s3://$BUCKET
	# Make the bucket accessible through website endpoints.
	echo "make $BUCKET accessible as a website"
	#aws s3 website s3://$BUCKET --index-document index.html --error-document jsearch/index.html
	s3conf=$(cat s3_website.json | envsubst)
	echo
	echo $s3conf
	echo
	aws s3api --profile $BUCKET put-bucket-website --bucket $BUCKET --website-configuration "$s3conf"
}

build_current_documentation() {
	mkdocs build
	cd site/
	gzip -9k -f search_content.json
	cd ..
}

upload_current_documentation() {
	src=site/
	dst=s3://$BUCKET$1

	cache=max-age=3600
	if [ "$NOCACHE" ]; then
		cache=no-cache
	fi

	echo
	echo "Uploading $src"
	echo "  to $dst"
	echo

	# a really complicated way to send only the files we want
	# if there are too many in any one set, aws s3 sync seems to fall over with 2 files to go
	#  versions.html_fragment
		include="--recursive --include \"*.$i\" "
		echo "uploading *.$i"
		run="aws s3 cp $src $dst $OPTIONS --profile $BUCKET --cache-control $cache --acl public-read $include"
		echo "======================="
		echo "$run"
		echo "======================="
		$run

	# Make sure the search_content.json.gz file has the right content-encoding
	aws s3 cp --profile $BUCKET --cache-control $cache --content-encoding="gzip" --acl public-read "site/search_content.json.gz" "$dst"
}

invalidate_cache() {
	if [ "" == "$DISTRIBUTION_ID" ]; then
		echo "Skipping Cloudfront cache invalidation"
		return
	fi

	dst=$1

	#aws cloudfront  create-invalidation --profile docs.docker.com --distribution-id $DISTRIBUTION_ID --invalidation-batch '{"Paths":{"Quantity":1, "Items":["'+$file+'"]},"CallerReference":"19dec2014sventest1"}'
	aws configure set preview.cloudfront true

	files=($(cat changed-files | grep 'sources/.*$' | sed -E 's#.*docs/sources##' | sed -E 's#index\.md#index.html#' | sed -E 's#\.md#/index.html#'))
	files[${#files[@]}]="/index.html"
	files[${#files[@]}]="/versions.html_fragment"

	len=${#files[@]}

	echo "aws cloudfront  create-invalidation --profile $AWS_S3_BUCKET --distribution-id $DISTRIBUTION_ID --invalidation-batch '" > batchfile
	echo "{\"Paths\":{\"Quantity\":$len," >> batchfile
	echo "\"Items\": [" >> batchfile

	#for file in $(cat changed-files | grep 'sources/.*$' | sed -E 's#.*docs/sources##' | sed -E 's#index\.md#index.html#' | sed -E 's#\.md#/index.html#')
	for file in "${files[@]}"
	do
		if [ "$file" == "${files[${#files[@]}-1]}" ]; then
			comma=""
		else
			comma=","
		fi
		echo "\"$dst$file\"$comma" >> batchfile
	done

	echo "]}, \"CallerReference\":" >> batchfile
	echo "\"$(date)\"}'" >> batchfile


	echo "-----"
	cat batchfile
	echo "-----"
	sh batchfile
	echo "-----"
}


if [ "$OPTIONS" != "--dryrun" ]; then
	setup_s3
fi

# Default to only building the version specific docs so we don't clober the latest by accident with old versions
if [ "$BUILD_ROOT" == "yes" ]; then
	echo "Building root documentation"
	build_current_documentation
	upload_current_documentation
	[ "$NOCACHE" ] || invalidate_cache
fi

#build again with /v1.0/ prefix
sed -i "s/^site_url:.*/site_url: \/$MAJOR_MINOR\//" mkdocs.yml
echo "Building the /$MAJOR_MINOR/ documentation"
build_current_documentation
upload_current_documentation "/$MAJOR_MINOR/"
[ "$NOCACHE" ] || invalidate_cache "/$MAJOR_MINOR"
