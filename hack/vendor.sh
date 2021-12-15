#!/usr/bin/env bash

# This file is just wrapper around 'go mod vendor' tool.
# For updating dependencies you should change `vendor.mod` file in root of the
# project.

set -e
set -x

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
${SCRIPTDIR}/go-mod-prepare.sh

if [ $# -eq 0 ] || [ "$1" != "archive/tar" ]; then
	GO111MODULE=auto go mod vendor -modfile vendor.mod
fi

if [ $# -eq 0 ] || [ "$1" = "archive/tar" ]; then
	echo "update vendored copy of archive/tar"
	: "${GO_VERSION:=$(awk -F '[ =]' '$1 == "ARG" && $2 == "GO_VERSION" { print $3; exit }' ./Dockerfile)}"
	rm -rf vendor/archive/tar
	mkdir -p vendor/archive/tar
	echo "downloading: https://golang.org/dl/go${GO_VERSION%.0}.src.tar.gz"
	curl -fsSL "https://golang.org/dl/go${GO_VERSION%.0}.src.tar.gz" \
		| tar --extract --gzip --directory=vendor/archive/tar --strip-components=4 go/src/archive/tar
	patch --strip=4 --directory=vendor/archive/tar --input="$PWD/patches/0001-archive-tar-do-not-populate-user-group-names.patch"
fi

