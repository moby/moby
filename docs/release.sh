#!/bin/bash
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
 You can also add NOCACHE=1 to publish without a cache, which is what we do for the master docs.
EOF
	exit 1
}

create_robots_txt() {
	cat > ./sources/robots.txt <<'EOF'
User-agent: *
Disallow: /
EOF
}

setup_s3() {
	# Try creating the bucket. Ignore errors (it might already exist).
	echo "create $BUCKET if it does not exist"
	aws s3 mb --profile $BUCKET s3://$BUCKET 2>/dev/null || true

	# Check access to the bucket.
	echo "test $BUCKET exists"
	aws s3 --profile $BUCKET ls s3://$BUCKET

	# Make the bucket accessible through website endpoints.
	echo "make $BUCKET accessible as a website"
	#aws s3 website s3://$BUCKET --index-document index.html --error-document jsearch/index.html
	local s3conf=$(cat s3_website.json | envsubst)
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

	printf "\nUploading $src to $dst\n"

	# a really complicated way to send only the files we want
	# if there are too many in any one set, aws s3 sync seems to fall over with 2 files to go
	#  versions.html_fragment
	include="--recursive --include \"*.$i\" "
	run="aws s3 cp $src $dst $OPTIONS --profile $BUCKET --cache-control $cache --acl public-read $include"
	printf "\n=====\n$run\n=====\n"
	$run

	# Make sure the search_content.json.gz file has the right content-encoding
	aws s3 cp --profile $BUCKET --cache-control $cache --content-encoding="gzip" --acl public-read "site/search_content.json.gz" "$dst"
}

invalidate_cache() {
	if [[ -z "$DISTRIBUTION_ID" ]]; then
		echo "Skipping Cloudfront cache invalidation"
		return
	fi

	dst=$1

	aws configure set preview.cloudfront true

	# Get all the files
	# not .md~ files
	# replace spaces w %20 so urlencoded
	files=( $(find site/ -not -name "*.md*" -type f | sed 's/site//g' | sed 's/ /%20/g') )

	len=${#files[@]}
	last_file=${files[$((len-1))]}

	echo "aws cloudfront  create-invalidation --profile $AWS_S3_BUCKET --distribution-id $DISTRIBUTION_ID --invalidation-batch '" > batchfile
	echo "{\"Paths\":{\"Quantity\":$len," >> batchfile
	echo "\"Items\": [" >> batchfile

	for file in "${files[@]}" ; do
		if [[ $file == $last_file ]]; then
			comma=""
		else
			comma=","
		fi
		echo "\"$dst$file\"$comma" >> batchfile
	done

	echo "]}, \"CallerReference\":\"$(date)\"}'" >> batchfile

	sh batchfile
}

main() {
	[ "$AWS_S3_BUCKET" ] || usage

	# Make sure there is an awsconfig file
	export AWS_CONFIG_FILE=$(pwd)/awsconfig
	[ -f "$AWS_CONFIG_FILE" ] || usage

	# Get the version
	VERSION=$(cat VERSION)

	# Disallow pushing dev docs to master
	if [ "$AWS_S3_BUCKET" == "docs.docker.com" ] && [ "${VERSION%-dev}" != "$VERSION" ]; then
		echo "Please do not push '-dev' documentation to docs.docker.com ($VERSION)"
		exit 1
	fi

	# Clean version - 1.0.2-dev -> 1.0
	export MAJOR_MINOR="v${VERSION%.*}"

	export BUCKET=$AWS_S3_BUCKET
	export AWS_DEFAULT_PROFILE=$BUCKET

	# debug variables
	echo "bucket: $BUCKET, full version: $VERSION, major-minor: $MAJOR_MINOR"
	echo "cfg file: $AWS_CONFIG_FILE ; profile: $AWS_DEFAULT_PROFILE"

	# create the robots.txt
	create_robots_txt

	if [ "$OPTIONS" != "--dryrun" ]; then
		setup_s3
	fi

	# Default to only building the version specific docs
	# so we don't clober the latest by accident with old versions
	if [ "$BUILD_ROOT" == "yes" ]; then
		echo "Building root documentation"
		build_current_documentation

		echo "Uploading root documentation"
		upload_current_documentation
		[ "$NOCACHE" ] || invalidate_cache
	fi

	#build again with /v1.0/ prefix
	sed -i "s/^site_url:.*/site_url: \/$MAJOR_MINOR\//" mkdocs.yml
	echo "Building the /$MAJOR_MINOR/ documentation"
	build_current_documentation

	echo "Uploading the documentation"
	upload_current_documentation "/$MAJOR_MINOR/"

	# Invalidating cache
	[ "$NOCACHE" ] || invalidate_cache "/$MAJOR_MINOR"
}

main
