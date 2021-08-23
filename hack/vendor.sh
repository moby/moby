#!/usr/bin/env bash

# This file is just wrapper around vndr (github.com/LK4D4/vndr) tool.
# For updating dependencies you should change `vendor.conf` file in root of the
# project. Please refer to https://github.com/LK4D4/vndr/blob/master/README.md for
# vndr usage.

set -e

if ! hash vndr; then
	echo "Please install vndr with \"go get github.com/LK4D4/vndr\" and put it in your \$GOPATH"
	exit 1
fi

if [ $# -eq 0 ] || [ "$1" = "archive/tar" ]; then
	echo "update vendored copy of archive/tar"
	: "${GO_VERSION:=$(awk -F '[ =]' '$1 == "ARG" && $2 == "GO_VERSION" { print $3; exit }' ./Dockerfile)}"
	rm -rf vendor/archive
	mkdir -p ./vendor/archive/tar
	echo "downloading: https://golang.org/dl/go${GO_VERSION%.0}.src.tar.gz"
	curl -fsSL "https://golang.org/dl/go${GO_VERSION%.0}.src.tar.gz" \
		| tar --extract --gzip --directory=vendor/archive/tar --strip-components=4 go/src/archive/tar
	patch --strip=4 --directory=vendor/archive/tar --input="$PWD/patches/0001-archive-tar-do-not-populate-user-group-names.patch"
fi

if [ $# -eq 0 ] || [ "$1" != "archive/tar" ]; then
	vndr -whitelist='^archive[/\\]tar' "$@"
fi
